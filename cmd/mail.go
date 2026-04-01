package cmd

import "github.com/spf13/cobra"

var mailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Manage local email capture and inspection services",
	Long: `Spin up a local SMTP catch-all server for testing transactional emails
without risk of sending to real users.

All outgoing mail from your application is captured and viewable via a
local web UI and a JSON API — no external service required.`,
}

func init() {
	rootCmd.AddCommand(mailCmd)
}
