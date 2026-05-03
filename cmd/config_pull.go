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

	"github.com/VitruvianSoftware/devx/internal/envvault"
	"github.com/spf13/cobra"
)

var configPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull team-managed `.env` secrets from vault providers securely.",
	Long: `Fetches secrets from configured 1Password, Bitwarden, or GCP vaults based on your devx.yaml.
The secrets are securely printed or loaded into your local environment runtime 
without being written as plaintext to the Mac's disk.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Idea 44: use resolveConfig so env sources from included projects are merged
		yamlPath, err := mustFindDevxConfig()
		if err != nil {
			return err
		}
		cfg, err := resolveConfig(yamlPath, "")
		if err != nil {
			return fmt.Errorf("could not read devx.yaml: %w", err)
		}

		if len(cfg.Env) == 0 {
			fmt.Println("No env vault sources configured in devx.yaml.")
			return nil
		}

		fmt.Printf("Fetching secrets from %d sources...\n", len(cfg.Env))

		secrets, err := envvault.PullAll(cfg.Env)
		if err != nil {
			return err
		}

		// Because we're required NOT to store them on disk automatically...
		// We'll output them safely to stdout using export statements so evaluating the CLI works.
		for k, v := range secrets {
			fmt.Printf("export %s=%q\n", k, v)
		}

		return nil
	},
}

func init() {
	configCmd.AddCommand(configPullCmd)
}
