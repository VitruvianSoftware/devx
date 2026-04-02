package cmd

import (
	"fmt"
	"os"

	"github.com/VitruvianSoftware/devx/internal/envvault"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local `.env` secrets up to team-managed remote vaults.",
	Long: `Reads your local .env file and pushes the contents to all the 
remote vault locations configured in your devx.yaml. 
This is helpful to migrate existing projects into remote vaults 
or to update remote shared values securely.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		yamlData, err := os.ReadFile("devx.yaml")
		if err != nil {
			return fmt.Errorf("could not read devx.yaml: %w", err)
		}

		type envConfig struct {
			Env []string `yaml:"env"`
		}

		var cfg envConfig
		if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
			return err
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
