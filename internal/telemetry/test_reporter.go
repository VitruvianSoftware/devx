package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// GoTestEvent mirrors the test2json output schema.
type GoTestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"` // in seconds
	Output  string    `json:"Output"`
}

// IsGoTestCmd returns true if the command is "go test".
func IsGoTestCmd(args []string) bool {
	if len(args) < 2 {
		return false
	}
	return args[0] == "go" && args[1] == "test"
}

// InjectJSONFlag returns a copy of args with -json inserted after "test"
// if not already present. It returns the original args if -json exists.
func InjectJSONFlag(args []string) []string {
	if !IsGoTestCmd(args) {
		return args
	}

	for _, arg := range args {
		if arg == "-json" || arg == "--json" {
			return args
		}
	}

	newArgs := make([]string, 0, len(args)+1)
	newArgs = append(newArgs, args[0], args[1], "-json")
	newArgs = append(newArgs, args[2:]...)
	return newArgs
}

// RunGoTestWithTelemetry runs the wrapped go test -json command,
// parses output, emits spans, and reconstructs stdout.
// logWriter, if non-nil, always receives full detailed output regardless of the detailed flag.
func RunGoTestWithTelemetry(args []string, dir string, stdout io.Writer, stderr io.Writer, detailed bool, logWriter io.Writer) (int, error) {
	newArgs := InjectJSONFlag(args)

	// Use discard writers if nil (e.g. non-verbose in ship.go)
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	cmd := exec.Command(newArgs[0], newArgs[1:]...)
	cmd.Dir = dir
	cmd.Stderr = stderr

	pipeReader, pipeWriter := io.Pipe()
	cmd.Stdout = pipeWriter

	err := cmd.Start()
	if err != nil {
		return 1, fmt.Errorf("failed to start test command: %w", err)
	}

	// Track test start times for accurate duration on fast tests
	testStarts := make(map[string]time.Time)

	// Counters for the summary line
	var totalPassed, totalFailed, totalSkipped int
	var pkgCount int

	// Buffer per-test output so we can dump it only on failure
	testOutputBuf := make(map[string][]string) // key: "pkg/TestName"

	// Use a WaitGroup to ensure the goroutine finishes processing before we return
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer pipeReader.Close()

		scanner := bufio.NewScanner(pipeReader)
		for scanner.Scan() {
			line := scanner.Bytes()

			var event GoTestEvent
			if err := json.Unmarshal(line, &event); err != nil {
				// Not JSON — compilation error or build failure. Always show.
				fmt.Fprintln(stderr, string(line))
				continue
			}

			switch event.Action {
			case "run":
				if event.Test != "" {
					testStarts[event.Package+"/"+event.Test] = event.Time
				}

			case "output":
				// Always write to log file if available (full detail)
				if logWriter != nil {
					fmt.Fprint(logWriter, event.Output)
				}

				if detailed {
					// Detailed mode: pass everything through verbatim
					fmt.Fprint(stdout, event.Output)
				} else {
					// Default mode: buffer test-level output for failure replay
					if event.Test != "" {
						key := event.Package + "/" + event.Test
						testOutputBuf[key] = append(testOutputBuf[key], event.Output)
					}
					// Suppress all stdout in default mode — only failures break through
				}

			case "pass", "fail", "skip":
				if event.Test == "" {
					// ── Package-level result ──
					pkgCount++
					if detailed {
						continue // detailed mode already printed everything
					}

					pkg := event.Package
					if idx := strings.Index(pkg, "/devx/"); idx != -1 {
						pkg = pkg[idx+6:]
					} else if strings.HasSuffix(pkg, "/devx") {
						pkg = "."
					}

					dur := time.Duration(event.Elapsed * float64(time.Second))

					switch event.Action {
					case "pass":
						fmt.Fprintf(stdout, "  ✓ %s (%s)\n", pkg, formatPkgDur(dur))
					case "fail":
						fmt.Fprintf(stdout, "  ✗ %s (%s)\n", pkg, formatPkgDur(dur))
					}
					continue
				}

				// ── Individual test result ──
				switch event.Action {
				case "pass":
					totalPassed++
				case "fail":
					totalFailed++
				case "skip":
					totalSkipped++
				}

				// Calculate duration
				dur := time.Duration(event.Elapsed * float64(time.Second))
				if dur == 0 {
					key := event.Package + "/" + event.Test
					if start, ok := testStarts[key]; ok {
						dur = event.Time.Sub(start)
					}
					if dur <= 0 {
						dur = time.Microsecond
					}
				}

				if detailed {
					// Detailed mode: print every test result
					symbol := "✓"
					if event.Action == "fail" {
						symbol = "✗"
					} else if event.Action == "skip" {
						symbol = "○"
					}
					durMs := dur.Milliseconds()
					if durMs == 0 {
						fmt.Fprintf(stdout, "%s %s  %s (<1ms)\n", symbol, strings.ToUpper(event.Action), event.Test)
					} else {
						fmt.Fprintf(stdout, "%s %s  %s (%dms)\n", symbol, strings.ToUpper(event.Action), event.Test, durMs)
					}
				} else if event.Action == "fail" {
					// Default mode: dump the buffered output for this failed test
					key := event.Package + "/" + event.Test
					fmt.Fprintf(stdout, "\n  ✗ FAIL %s\n", event.Test)
					if buf, ok := testOutputBuf[key]; ok {
						for _, line := range buf {
							fmt.Fprintf(stdout, "    %s", line)
						}
					}
					fmt.Fprintln(stdout)
				}

				// Clean up buffer
				key := event.Package + "/" + event.Test
				delete(testOutputBuf, key)

				// Emit telemetry span (always, regardless of mode)
				projectName := os.Getenv("DEVX_PROJECT")
				if projectName == "" {
					projectName = "unknown"
				}

				spanName := "go_test"
				if event.Test != "" {
					spanName = "go_test: " + event.Test
				} else {
					// Default to package name for package-level events
					spanName = "go_test: " + event.Package
				}

				ExportSpan(spanName, dur,
					Attr("devx.test.name", event.Test),
					Attr("devx.test.package", event.Package),
					Attr("devx.test.status", event.Action),
					Attr("devx.project", projectName),
				)
			}
		}
	}()

	err = cmd.Wait()
	pipeWriter.Close()
	wg.Wait() // Wait for all output processing to complete

	// Print summary line in default mode
	if !detailed && totalPassed+totalFailed+totalSkipped > 0 {
		parts := []string{}
		if totalPassed > 0 {
			parts = append(parts, fmt.Sprintf("%d passed", totalPassed))
		}
		if totalFailed > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", totalFailed))
		}
		if totalSkipped > 0 {
			parts = append(parts, fmt.Sprintf("%d skipped", totalSkipped))
		}
		fmt.Fprintf(stdout, "  %d packages, %s\n", pkgCount, strings.Join(parts, ", "))
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return exitCode, err
}

// formatPkgDur formats a package test duration for display.
func formatPkgDur(d time.Duration) string {
	if d < time.Millisecond {
		return "<1ms"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
