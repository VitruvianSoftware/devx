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

	"github.com/VitruvianSoftware/devx/internal/secrets"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Add or rotate credentials stored in .env",
	Long:  "Opens an interactive form to set or update the Cloudflare token and hostname.\nValues are saved to the --env-file path (default: .env).",
	RunE:  runSecrets,
}

func runSecrets(_ *cobra.Command, _ []string) error {
	if err := secrets.Rotate(envFile); err != nil {
		return fmt.Errorf("secrets: %w", err)
	}
	fmt.Printf("✓ Credentials saved to %s\n", envFile)
	return nil
}

func init() {
	configCmd.AddCommand(secretsCmd)
}
