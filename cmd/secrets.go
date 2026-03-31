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

