package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/spf13/cobra"
)

var dbRestartRuntime string

var dbRestartCmd = &cobra.Command{
	Use:   "restart <engine>",
	Short: "Restart a devx-managed database container (volume data is preserved)",
	Long: fmt.Sprintf(`Restart a running devx-managed database container without destroying its volume.
Useful when a container becomes unresponsive or after a host reboot.

Supported engines: %s`, strings.Join(database.SupportedEngines(), ", ")),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engineName := strings.ToLower(args[0])
		if _, ok := database.Registry[engineName]; !ok {
			return fmt.Errorf("unknown engine %q — supported: %s",
				engineName, strings.Join(database.SupportedEngines(), ", "))
		}

		runtime := dbRestartRuntime
		if runtime != "podman" && runtime != "docker" {
			return fmt.Errorf("unsupported runtime %q — use 'podman' or 'docker'", runtime)
		}

		containerName := fmt.Sprintf("devx-db-%s", engineName)
		fmt.Printf("🔄 Restarting %s (%s)...\n", engineName, containerName)

		if err := exec.Command(runtime, "restart", containerName).Run(); err != nil {
			return fmt.Errorf("failed to restart %s: %w", engineName, err)
		}

		fmt.Printf("✓ %s restarted successfully\n", engineName)
		return nil
	},
}

func init() {
	dbRestartCmd.Flags().StringVar(&dbRestartRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	dbCmd.AddCommand(dbRestartCmd)
}
