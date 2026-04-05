package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/telemetry"
)

var actionList bool

var actionCmd = &cobra.Command{
	Use:   "action [name]",
	Short: "Run a named custom action defined in devx.yaml",
	Long: `Execute on-demand tasks declared under 'customActions:' in devx.yaml.

Custom actions group multiple sequential commands under a single name,
ideal for repetitive workflows like seeding data, generating code, or
running a full local CI suite.

Examples:
  devx action ci           # run the 'ci' custom action
  devx action seed-db      # run database seeding
  devx action --list       # list all available actions
  devx action ci --dry-run # preview commands without executing

Define actions in devx.yaml:
  customActions:
    ci:
      commands:
        - ["go", "test", "./..."]
        - ["go", "vet", "./..."]
        - ["go", "build", "./..."]
    seed-db:
      command: ["npm", "run", "db:seed"]`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAction,
}

func runAction(_ *cobra.Command, args []string) error {
	// Load config
	cfg, err := resolveConfig("devx.yaml", "")
	if err != nil {
		if actionList {
			fmt.Println("No devx.yaml found — no custom actions available.")
			return nil
		}
		return fmt.Errorf("cannot load devx.yaml: %w", err)
	}

	if len(cfg.CustomActions) == 0 {
		if actionList {
			fmt.Println("No custom actions defined in devx.yaml.")
			return nil
		}
		return fmt.Errorf("no customActions defined in devx.yaml")
	}

	// --list mode
	if actionList {
		names := make([]string, 0, len(cfg.CustomActions))
		for name := range cfg.CustomActions {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Println("Available custom actions:")
		for _, name := range names {
			action := cfg.CustomActions[name]
			cmds := action.Cmds()
			fmt.Printf("  %-20s  (%d command%s)\n", name, len(cmds), pluralize(len(cmds)))
			for _, cmdArgs := range cmds {
				fmt.Printf("    - %s\n", strings.Join(cmdArgs, " "))
			}
		}
		return nil
	}

	if len(args) == 0 {
		// No action name given — list available and hint
		names := make([]string, 0, len(cfg.CustomActions))
		for name := range cfg.CustomActions {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Println("Usage: devx action <name>")
		fmt.Println()
		fmt.Println("Available actions:")
		for _, name := range names {
			fmt.Printf("  %s\n", name)
		}
		os.Exit(1)
	}

	actionName := args[0]
	action, ok := cfg.CustomActions[actionName]
	if !ok {
		names := make([]string, 0, len(cfg.CustomActions))
		for name := range cfg.CustomActions {
			names = append(names, name)
		}
		sort.Strings(names)
		return fmt.Errorf("action %q not found — available actions: %s", actionName, strings.Join(names, ", "))
	}

	cmds := action.Cmds()
	if len(cmds) == 0 {
		return fmt.Errorf("action %q has no commands defined", actionName)
	}

	// ── Dry-run mode ────────────────────────────────────────────────────
	if DryRun {
		fmt.Printf("Would run action %q (%d command%s):\n", actionName, len(cmds), pluralize(len(cmds)))
		for i, cmd := range cmds {
			fmt.Printf("  %d. %s\n", i+1, strings.Join(cmd, " "))
		}
		return nil
	}

	// ── Execute ─────────────────────────────────────────────────────────
	if !outputJSON {
		fmt.Printf("▸ devx action: %s\n", actionName)
	}

	cwd, _ := os.Getwd()

	// Log routing
	logDir := filepath.Join(os.Getenv("HOME"), ".devx", "logs")
	_ = os.MkdirAll(logDir, 0755)

	logFile, logErr := os.OpenFile(filepath.Join(logDir, "action-"+actionName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if logErr != nil {
		fmt.Printf("Warning: Could not open log file: %v\n", logErr)
	} else {
		defer logFile.Close()
	}

	var outWriter, errWriter io.Writer
	if logFile != nil {
		outWriter = io.MultiWriter(os.Stdout, logFile)
		errWriter = io.MultiWriter(os.Stderr, logFile)
	} else {
		outWriter = os.Stdout
		errWriter = os.Stderr
	}

	start := time.Now()
	var exitCode int

	for i, cmdArgs := range cmds {
		if !outputJSON {
			fmt.Printf("  [%d/%d] %s\n", i+1, len(cmds), strings.Join(cmdArgs, " "))
		}

		// Go test interception for granular telemetry
		if telemetry.IsGoTestCmd(cmdArgs) {
			// For go test: terminal gets clean summary, log file gets full detail
			var testLog io.Writer
			if logFile != nil {
				testLog = logFile
			}
			code, _ := telemetry.RunGoTestWithTelemetry(cmdArgs, cwd, os.Stdout, os.Stderr, DetailedOutput, testLog)
			if code != 0 {
				exitCode = code
				break
			}
			continue
		}
		command := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		command.Dir = cwd
		command.Stdout = outWriter
		command.Stderr = errWriter
		command.Stdin = os.Stdin

		cmdStart := time.Now()
		if err := command.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start %q: %v\n", strings.Join(cmdArgs, " "), err)
			exitCode = 1
			break
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			_ = command.Process.Signal(syscall.SIGINT)
		}()

		if err := command.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
			signal.Stop(sigChan)
			
			// Always record the failure span before breaking
			cmdDuration := time.Since(cmdStart)
			telemetry.RecordEvent("devx_run", cmdDuration,
				telemetry.Attr("devx.command", strings.Join(cmdArgs, " ")),
				telemetry.Attr("devx.exit_code", exitCode),
			)
			break
		}
		signal.Stop(sigChan)

		// Record successful command run telemetry
		cmdDuration := time.Since(cmdStart)
		telemetry.RecordEvent("devx_run", cmdDuration,
			telemetry.Attr("devx.command", strings.Join(cmdArgs, " ")),
			telemetry.Attr("devx.exit_code", exitCode),
		)
	}

	duration := time.Since(start)

	// ── Telemetry ───────────────────────────────────────────────────────
	projName := actionName
	if cfg.Name != "" {
		projName = cfg.Name
	}

	telemetry.RecordEvent("devx_action", duration,
		telemetry.Attr("devx.action.name", actionName),
		telemetry.Attr("devx.action.exit_code", exitCode),
		telemetry.Attr("devx.action.command_count", len(cmds)),
		telemetry.Attr("devx.project", projName),
	)

	// ── Output ──────────────────────────────────────────────────────────
	if outputJSON {
		result := map[string]interface{}{
			"action":      actionName,
			"exit_code":   exitCode,
			"duration_ms": duration.Milliseconds(),
			"commands":    len(cmds),
		}
		enc, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(enc))
	} else {
		status := "✓"
		if exitCode != 0 {
			status = "✗"
		}
		fmt.Printf("\n%s  action %q completed in %s (exit %d)\n", status, actionName, duration.Round(time.Millisecond), exitCode)
	}

	telemetry.Flush()

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func init() {
	rootCmd.AddCommand(actionCmd)
	actionCmd.Flags().BoolVar(&actionList, "list", false, "List all available custom actions")
}
