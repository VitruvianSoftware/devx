package cmd

import (
	"fmt"
	"os/exec"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var mailRmRuntime string

var mailRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Stop and remove the devx-managed mail catcher",
	RunE:  runMailRm,
}

func init() {
	mailRmCmd.Flags().StringVar(&mailRmRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	mailCmd.AddCommand(mailRmCmd)
}

func runMailRm(_ *cobra.Command, _ []string) error {
	runtime := mailRmRuntime

	if DryRun {
		fmt.Printf("[dry-run] Would stop and remove container %s\n", mailContainerName)
		return nil
	}

	if !NonInteractive {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Remove MailHog mail catcher?").
					Description(fmt.Sprintf("This will stop and remove container '%s'.\nAll captured emails will be lost.", mailContainerName)).
					Affirmative("Yes, remove it").
					Negative("Cancel").
					Value(&confirmed),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Printf("Stopping %s...\n", mailContainerName)
	_ = exec.Command(runtime, "stop", mailContainerName).Run()

	fmt.Printf("Removing %s...\n", mailContainerName)
	if err := exec.Command(runtime, "rm", "-f", mailContainerName).Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	fmt.Println("✅ Mail catcher removed.")
	return nil
}
