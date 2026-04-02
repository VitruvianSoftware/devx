package cmd

import "github.com/spf13/cobra"

var mockCmd = &cobra.Command{
	Use:   "mock",
	Short: "Manage local OpenAPI mock servers powered by Stoplight Prism",
	Long: `Spin up intelligent OpenAPI mock servers that simulate 3rd-party APIs
(Stripe, Twilio, internal services) locally based on remote OpenAPI specs.

Mock servers run as persistent background containers and are accessible
via injected environment variables (e.g. MOCK_STRIPE_URL=http://localhost:4010).`,
}

func init() {
	rootCmd.AddCommand(mockCmd)
}
