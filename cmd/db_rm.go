package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/database"
)

var rmRuntime string
var rmKeepVolume bool

var dbRmCmd = &cobra.Command{
	Use:   "rm <engine>",
	Short: "Stop and remove a devx-managed database",
	Long:  `Removes the database container. Use --keep-volume to preserve data for later.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDbRm,
}

func init() {
	dbRmCmd.Flags().StringVar(&rmRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	dbRmCmd.Flags().BoolVar(&rmKeepVolume, "keep-volume", false,
		"Keep the persistent data volume (only remove the container)")
	dbCmd.AddCommand(dbRmCmd)
}

func runDbRm(_ *cobra.Command, args []string) error {
	engineName := strings.ToLower(args[0])
	if _, ok := database.Registry[engineName]; !ok {
		return fmt.Errorf("unknown engine %q — supported: %s",
			engineName, strings.Join(database.SupportedEngines(), ", "))
	}

	runtime := rmRuntime
	containerName := fmt.Sprintf("devx-db-%s", engineName)
	volumeName := fmt.Sprintf("devx-data-%s", engineName)

	if !rmKeepVolume {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Remove %s and its data?", containerName)).
					Description(fmt.Sprintf("This will delete the container AND the volume '%s'.\nAll data will be permanently lost.", volumeName)).
					Affirmative("Yes, delete everything").
					Negative("Cancel").
					Value(&confirmed),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Removal cancelled.")
			return nil
		}
	}

	fmt.Printf("Stopping %s...\n", containerName)
	_ = exec.Command(runtime, "stop", containerName).Run()

	fmt.Printf("Removing container %s...\n", containerName)
	_ = exec.Command(runtime, "rm", "-f", containerName).Run()

	if !rmKeepVolume {
		fmt.Printf("Removing volume %s...\n", volumeName)
		_ = exec.Command(runtime, "volume", "rm", "-f", volumeName).Run()
		fmt.Println("✓ Container and data volume removed.")
	} else {
		fmt.Printf("✓ Container removed. Volume '%s' preserved.\n", volumeName)
	}

	return nil
}
