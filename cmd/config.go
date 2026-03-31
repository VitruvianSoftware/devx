package cmd

import "github.com/spf13/cobra"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage devx configuration and credentials",
	Long:  `Commands for managing secrets, environment files, and devx settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
