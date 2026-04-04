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
func RunGoTestWithTelemetry(args []string, dir string, stdout io.Writer, stderr io.Writer) (int, error) {
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
				// Not JSON — compilation error or build failure. Pipe to stderr.
				fmt.Fprintln(stderr, string(line))
				continue
			}

			switch event.Action {
			case "run":
				// Track start time for duration measurement
				if event.Test != "" {
					testStarts[event.Package+"/"+event.Test] = event.Time
				}

			case "output":
				fmt.Fprint(stdout, event.Output)

			case "pass", "fail", "skip":
				if event.Test == "" {
					// Package-level event — skip telemetry
					continue
				}

				symbol := "✓"
				if event.Action == "fail" {
					symbol = "✗"
				} else if event.Action == "skip" {
					symbol = "○"
				}

				// Calculate duration: use Elapsed if available, else wall clock
				dur := time.Duration(event.Elapsed * float64(time.Second))
				if dur == 0 {
					// Fast tests report 0.00s. Use our tracked start time instead.
					key := event.Package + "/" + event.Test
					if start, ok := testStarts[key]; ok {
						dur = event.Time.Sub(start)
					}
					// Floor at 1µs so the span has non-zero width
					if dur <= 0 {
						dur = time.Microsecond
					}
				}

				durMs := dur.Milliseconds()
				if durMs == 0 {
					fmt.Fprintf(stdout, "%s %s  %s (<1ms)\n", symbol, strings.ToUpper(event.Action), event.Test)
				} else {
					fmt.Fprintf(stdout, "%s %s  %s (%dms)\n", symbol, strings.ToUpper(event.Action), event.Test, durMs)
				}

				// Emit telemetry span
				projectName := os.Getenv("DEVX_PROJECT")
				if projectName == "" {
					projectName = "unknown"
				}

				ExportSpan("go_test", dur,
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
