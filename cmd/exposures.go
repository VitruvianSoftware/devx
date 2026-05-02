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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/spf13/cobra"
)

var exposuresCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"exposures", "ls"},
	Short:   "List active port exposures",
	RunE: func(cmd *cobra.Command, args []string) error {
		devName := os.Getenv("USER")

		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil {
			if s.DevHostname != "" {
				cfg.DevHostname = s.DevHostname
			}
		}

		if err := ensureCloudflareLogin(); err != nil {
			return err
		}

		tunnels, err := cloudflare.ListExposedTunnels(devName)
		if err != nil {
			return fmt.Errorf("failed listing tunnels: %w", err)
		}

		if len(tunnels) == 0 {
			if outputJSON {
				fmt.Println("[]")
				return nil
			}
			fmt.Println("No active exposures found. Run 'devx tunnel expose [port]' to create one.")
			return nil
		}

		if !outputJSON {
			fmt.Println("🌍 Active Cloudflare Exposures:")
		}

		type tunnelJSON struct {
			Name      string `json:"name"`
			URL       string `json:"url"`
			Port      string `json:"port"`
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
		}
		var listJSON []tunnelJSON

		prefix := fmt.Sprintf("devx-expose-%s-", devName)
		for _, t := range tunnels {
			exposeID := strings.TrimPrefix(t.Name, prefix)
			fullDomain := exposure.GenerateDomain(exposeID, cfg.CFDomain)
			port := exposure.LookupPort(t.Name)
			if port == "" {
				port = "unknown"
			}
			if outputJSON {
				listJSON = append(listJSON, tunnelJSON{
					Name:      t.Name,
					URL:       "https://" + fullDomain,
					Port:      port,
					ID:        t.ID,
					CreatedAt: string(t.CreatedAt),
				})
				continue
			}

			fmt.Printf("  • %s\n", t.Name)
			fmt.Printf("    URL:      https://%s\n", fullDomain)
			fmt.Printf("    Port:     %s\n", port)
			fmt.Printf("    ID:       %s\n", t.ID)
			fmt.Printf("    Created:  %s\n\n", t.CreatedAt)
		}

		if outputJSON {
			enc, _ := json.MarshalIndent(listJSON, "", "  ")
			fmt.Println(string(enc))
		}

		return nil
	},
}

func init() {
	tunnelCmd.AddCommand(exposuresCmd)
}
