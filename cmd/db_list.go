package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var listRuntime string

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devx-managed databases",
	Aliases: []string{"ls"},
	RunE:  runDbList,
}

func init() {
	dbListCmd.Flags().StringVar(&listRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	dbCmd.AddCommand(dbListCmd)
}

func runDbList(_ *cobra.Command, _ []string) error {
	runtime := listRuntime

	// List all containers with devx labels
	cmd := exec.Command(runtime, "ps", "-a",
		"--filter", "label=managed-by=devx",
		"--filter", "label=devx-engine",
		"--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.Labels}}")

	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		if outputJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Println(tui.StyleMuted.Render("  No databases running. Use 'devx db spawn <engine>' to start one."))
		fmt.Printf("  Supported: %s\n", strings.Join(database.SupportedEngines(), ", "))
		return nil
	}

	if !outputJSON {
		fmt.Println(tui.StyleTitle.Render("devx — Databases") + "\n")
	}

	type dbJSON struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Ports  string `json:"ports"`
		Engine string `json:"engine"`
	}
	var dbList []dbJSON

	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 3 {
			continue
		}
		name := parts[0]
		status := parts[1]
		ports := parts[2]

		statusStyle := tui.StyleDetailDone
		if !strings.Contains(strings.ToLower(status), "up") {
			statusStyle = tui.StyleDetailError
		}

		engineName := strings.TrimPrefix(name, "devx-db-")
		engine, ok := database.Registry[engineName]
		displayName := engineName
		if ok {
			displayName = engine.Name
		}

		if outputJSON {
			dbList = append(dbList, dbJSON{
				Name:   name,
				Status: status,
				Ports:  ports,
				Engine: displayName,
			})
			continue
		}

		fmt.Printf("  %s  %s  %s  %s\n",
			tui.StyleLabel.Render(displayName),
			tui.StyleStepName.Render(name),
			statusStyle.Render(status),
			tui.StyleMuted.Render(ports),
		)
	}

	if outputJSON {
		enc, _ := json.MarshalIndent(dbList, "", "  ")
		fmt.Println(string(enc))
	} else {
		fmt.Println()
	}

	return nil
}
