package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/telemetry"
	"github.com/VitruvianSoftware/devx/internal/tui"
	"github.com/spf13/cobra"
)

var traceListRuntime string

var traceListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all devx-managed telemetry backends",
	Aliases: []string{"ls"},
	RunE:    runTraceList,
}

func init() {
	traceListCmd.Flags().StringVar(&traceListRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	traceCmd.AddCommand(traceListCmd)
}

func runTraceList(_ *cobra.Command, _ []string) error {
	runtime := traceListRuntime

	out, err := exec.Command(runtime, "ps", "-a",
		"--filter", "label=managed-by=devx",
		"--filter", "label=devx-telemetry",
		"--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}",
	).Output()
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		fmt.Println(tui.StyleMuted.Render("  No telemetry backends running. Use 'devx trace spawn [jaeger|grafana]' to start one."))
		return nil
	}

	fmt.Println(tui.StyleTitle.Render("devx — Telemetry Backends") + "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		name, status := parts[0], parts[1]
		ports := ""
		if len(parts) == 3 {
			ports = parts[2]
		}

		statusStyle := tui.StyleDetailDone
		if !strings.Contains(strings.ToLower(status), "up") {
			statusStyle = tui.StyleDetailError
		}

		// Derive the engine from the container name
		engineLabel := strings.TrimPrefix(name, "devx-telemetry-")
		for _, e := range telemetry.SupportedEngines {
			if string(e) == engineLabel {
				engineLabel = string(e)
				break
			}
		}

		fmt.Printf("  %s  %s  %s  %s\n",
			tui.StyleLabel.Render(engineLabel),
			tui.StyleStepName.Render(name),
			statusStyle.Render(status),
			tui.StyleMuted.Render(ports),
		)
	}
	fmt.Println()
	return nil
}
