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
		cfg, err := resolveConfig("devx.yaml", "")
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
