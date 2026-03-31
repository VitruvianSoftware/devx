package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/config"
)

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Stop and remove the dev VM (destructive)",
	RunE:  runTeardown,
}

var forceFlag bool

func init() {
	teardownCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Skip confirmation prompt")
	vmCmd.AddCommand(teardownCmd)
}

func runTeardown(_ *cobra.Command, _ []string) error {
	vm, err := getVMProvider()
	if err != nil {
		return err
	}

	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, "", "", "")

	if !forceFlag && !NonInteractive {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Destroy %q?", cfg.DevHostname)).
					Description(fmt.Sprintf("This will stop and permanently delete the VM (%s) and all its data.\nTailscale re-authentication will be required on next setup.", vm.Name())).
					Affirmative("Yes, destroy it").
					Negative("Cancel").
					Value(&confirmed),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Teardown cancelled.")
			return nil
		}
	}

	fmt.Printf("Stopping %s (%s)...\n", cfg.DevHostname, vm.Name())
	_ = vm.StopAll()

	fmt.Printf("Removing %s...\n", cfg.DevHostname)
	_ = vm.Remove(cfg.DevHostname)

	fmt.Println("✓ VM removed.")
	return nil
}
