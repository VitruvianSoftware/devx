package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var vmSleepCmd = &cobra.Command{
	Use:   "sleep",
	Short: "Put the VM to sleep (pause resources)",
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, err := getVMProvider()
		if err != nil {
			return err
		}

		if prov.Name() != "podman" {
			fmt.Printf("✓ %s provider natively manages background sleep state.\n", prov.Name())
			return nil
		}

		fmt.Println("Putting VM to sleep...")
		if err := prov.Sleep("devx"); err != nil {
			return fmt.Errorf("failed to sleep: %w", err)
		}

		fmt.Println("✓ VM is asleep.")
		return nil
	},
}

func init() {
	vmCmd.AddCommand(vmSleepCmd)
}
