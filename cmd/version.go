package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is set at build time via ldflags.
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the devx version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("devx", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
