package cmd

import "github.com/spf13/cobra"

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage intelligent file synchronization into containers",
	Long: `Bypass slow VirtioFS volume mounts by syncing file changes directly
into running containers via Mutagen. Changes propagate in milliseconds.

Sync sessions run as persistent background processes and survive terminal exit.
Use 'devx sync rm' to clean up when finished.

Quick start:
  devx sync up          # start all sync sessions from devx.yaml
  devx sync list        # show active sessions
  devx sync rm          # terminate all sessions`,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
