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
	"os"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/spf13/cobra"
)

var unexposeCmd = &cobra.Command{
	Use:   "unexpose",
	Short: "Clean up all exposed localhost ports routed through Cloudflare.",
	RunE: func(cmd *cobra.Command, args []string) error {
		devName := os.Getenv("USER")

		if err := ensureCloudflareLogin(); err != nil {
			return err
		}

		fmt.Println("🧹 Cleaning up exposed Cloudflare applications...")

		if DryRun {
			fmt.Println("DRY RUN: Would list and delete all active tunnels matching devx-expose-" + devName + "-*")
			return nil
		}

		count, err := cloudflare.CleanupExposedTunnels(devName)
		if err != nil {
			return fmt.Errorf("failed cleaning tunnels: %w", err)
		}

		fmt.Printf("✓ Successfully removed %d exposed tunnels.\n", count)
		_ = exposure.RemoveAll()
		return nil
	},
}

func init() {
	tunnelCmd.AddCommand(unexposeCmd)
}
