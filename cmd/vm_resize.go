// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
