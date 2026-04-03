package cmd

import (
	"fmt"
	"os"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/state"
	"github.com/spf13/cobra"
)

var stateDumpFile string

var stateDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Snapshot topology, dependencies, and crashing container logs into a Shareable Diagnostic Dump",
	Long: `Outputs a meticulously structured, redact-safe snapshot of the running environment.
Useful for sharing context in bug trackers or 'it doesn't work on my machine' discussions.
Generates output in Markdown to stdout by default, or structured JSON.`,
	RunE: runStateDump,
}

func runStateDump(_ *cobra.Command, _ []string) error {
	vm, err := getVMProvider()
	if err != nil {
		return err
	}

	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, "", "", "")

	vmState := "not created"
	if info, err := vm.Inspect(cfg.DevHostname); err == nil {
		vmState = info.State
	}

	tsStatus := "down"
	if vm.IsRunning(cfg.DevHostname) {
		tsStatus = "ok" // generic simplification for the overall state dump
	}

	report, err := state.GenerateDump(cfg, vm.Name(), vmState, tsStatus)
	if err != nil {
		return fmt.Errorf("failed to generate state dump: %w", err)
	}

	var output string
	if outputJSON {
		output = state.GenerateJSON(report)
	} else {
		output = state.GenerateMarkdown(report)
	}

	if stateDumpFile != "" {
		if err := os.WriteFile(stateDumpFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("could not write dump to file: %w", err)
		}
		fmt.Printf("✅ Diagnostic state dump written to %s\n", stateDumpFile)
		return nil
	}

	fmt.Println(output)
	return nil
}

func init() {
	stateDumpCmd.Flags().StringVarP(&stateDumpFile, "file", "f", "", "Output directly to a file (e.g. /tmp/dump.md)")
	stateCmd.AddCommand(stateDumpCmd)
}
