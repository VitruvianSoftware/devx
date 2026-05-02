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

	"github.com/VitruvianSoftware/devx/internal/envvault"
	"github.com/spf13/cobra"
)

var configPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local `.env` secrets up to team-managed remote vaults.",
	Long: `Reads your local .env file and pushes the contents to all the 
remote vault locations configured in your devx.yaml. 
This is helpful to migrate existing projects into remote vaults 
or to update remote shared values securely.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Idea 44: use resolveConfig so env sources from included projects are merged
		cfg, err := resolveConfig("devx.yaml", "")
		if err != nil {
			return fmt.Errorf("could not read devx.yaml: %w", err)
		}

		if len(cfg.Env) == 0 {
			fmt.Println("No env vault sources configured in devx.yaml to push to.")
			return nil
		}

		dotEnvRaw, err := os.ReadFile(".env")
		if err != nil {
			return fmt.Errorf("could not read local .env file: %w", err)
		}

		fmt.Printf("Pushing local .env to configured vaults...\n")

		if err := envvault.PushAll(cfg.Env, dotEnvRaw); err != nil {
			return err
		}

		fmt.Println("✓ Successfully pushed local secrets to remote vaults.")

		return nil
	},
}

func init() {
	configCmd.AddCommand(configPushCmd)
}
