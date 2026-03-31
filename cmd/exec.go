package cmd

import "github.com/spf13/cobra"

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Run low-level infrastructure tools directly",
	Long: `Pass-through wrappers for the underlying tools that power devx.
These commands execute the native binaries with your arguments forwarded as-is.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
