package cmd

import (
	"github.com/spf13/cobra"
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Local CI pipeline emulation",
	Long: `Execute GitHub Actions workflows locally inside isolated containers.

devx ci run parses .github/workflows/ and executes run: shell blocks
inside Podman/Docker containers, giving you ~80% parity with GitHub Actions
without leaving your terminal.

LIMITATION: uses: actions (like actions/setup-go or actions/upload-artifact)
are intentionally skipped. Only run: shell blocks are executed. This is a
deliberate design decision — see 'devx ci run --help' for details.`,
}

func init() {
	rootCmd.AddCommand(ciCmd)
}
