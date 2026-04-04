package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/telemetry"
)

var runName string

var runCmd = &cobra.Command{
	Use:   "run -- [command...]",
	Short: "Run a command with telemetry, secret injection, and log routing",
	Long: `Wraps an arbitrary host command with devx telemetry and log routing.
Timing, exit code, and command metadata are recorded locally and exported
as OTel spans to any running trace backend.

Familiar to Docker users — devx run works like docker run for local commands.

Examples:
  devx run -- npm test
  devx run -- go build ./...
  devx run -- make deploy
  devx run --name api -- go run ./cmd/api

Global flags are parsed before '--':
  devx run --dry-run -- npm test    # prints intent without executing`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := runName
		if name == "" {
			name = filepath.Base(args[0])
		}
		cmdDisplay := strings.Join(args, " ")

		// ── Dry-run mode ────────────────────────────────────────────
		if DryRun {
			fmt.Printf("Would run: %s\n", cmdDisplay)
			return nil
		}

		// Ensure log dir exists
		logDir := filepath.Join(os.Getenv("HOME"), ".devx", "logs")
		_ = os.MkdirAll(logDir, 0755)

		// Open log file for append
		logFile, err := os.OpenFile(filepath.Join(logDir, name+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Printf("Warning: Could not open log file: %v\n", err)
		} else {
			defer logFile.Close()
		}

		if !outputJSON {
			fmt.Printf("▸ devx run: %s\n", cmdDisplay)
		}

		cwd, _ := os.Getwd()
		command := exec.Command(args[0], args[1:]...)
		command.Dir = cwd

		// Setup multi-writers to route to both terminal and log file
		if logFile != nil {
			command.Stdout = io.MultiWriter(os.Stdout, logFile)
			command.Stderr = io.MultiWriter(os.Stderr, logFile)
		} else {
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
		}
		command.Stdin = os.Stdin

		// ── Execute with timing ─────────────────────────────────────
		start := time.Now()

		err = command.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start process: %v\n", err)
			os.Exit(1)
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			_ = command.Process.Signal(syscall.SIGINT)
		}()

		err = command.Wait()
		duration := time.Since(start)

		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		// ── Record telemetry ────────────────────────────────────────
		projName := name
		if cfg, cfgErr := resolveConfig("devx.yaml", ""); cfgErr == nil && cfg.Name != "" {
			projName = cfg.Name
		}

		telemetry.RecordEvent("devx_run", duration,
			telemetry.Attr("devx.command", cmdDisplay),
			telemetry.Attr("devx.exit_code", exitCode),
			telemetry.Attr("devx.project", projName),
		)

		if !outputJSON {
			fmt.Printf("\n⏱  %s completed in %s (exit %d)\n", cmdDisplay, duration.Round(time.Millisecond), exitCode)
		}

		telemetry.Flush() // Wait for OTLP export to complete

		// Propagate the child's exit code
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runName, "name", "n", "", "Custom name for the log stream (defaults to executable name)")
}
