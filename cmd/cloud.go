package cmd

import "github.com/spf13/cobra"

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Spawn local GCP cloud service emulators",
	Long: `Emulate GCP cloud services (GCS, Pub/Sub, Firestore, etc.) locally inside
the devx VM. The emulator endpoint URLs are printed on stdout so you can
copy them directly into your .env — or inject them automatically via devx shell.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(cloudCmd)
}
