package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var cpus int
var memoryMB int

var vmResizeCmd = &cobra.Command{
	Use:   "resize",
	Short: "Dynamically resize VM RAM and CPU allocation",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cpus == 0 && memoryMB == 0 {
			return fmt.Errorf("must specify either --cpus (-c) or --memory (-m)")
		}

		prov, err := getVMProvider()
		if err != nil {
			return err
		}

		if prov.Name() != "podman" {
			return fmt.Errorf("resize is only supported on Podman. Docker/Orbstack manage resources dynamically.")
		}

		fmt.Printf("Resizing %s VM to %d CPUs, %d MB RAM...\n", prov.Name(), cpus, memoryMB)

		if err := prov.Resize("devx", cpus, memoryMB); err != nil {
			return err
		}

		fmt.Println("✓ Resize complete.")
		return nil
	},
}

func init() {
	vmResizeCmd.Flags().IntVarP(&cpus, "cpus", "c", 0, "Number of virtual CPUs")
	vmResizeCmd.Flags().IntVarP(&memoryMB, "memory", "m", 0, "Amount of memory in megabytes")
	vmCmd.AddCommand(vmResizeCmd)
}
