package cmd

import "github.com/spf13/cobra"

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage local development databases",
	Long:  `Provision, list, and remove local databases with persistent volumes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
}
