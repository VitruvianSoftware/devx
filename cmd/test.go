package cmd

import "github.com/spf13/cobra"

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run isolated test topologies against ephemeral environments",
}

func init() {
	rootCmd.AddCommand(testCmd)
}
