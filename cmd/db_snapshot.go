package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage point-in-time snapshots of local database volumes",
	Long: `Create, restore, list, and delete point-in-time snapshots of devx-managed
database volumes. Snapshots are stored as tar archives in ~/.devx/snapshots/.

Useful before running destructive migrations or testing complex state changes —
restore to a known-good state in seconds without re-running SQL seed scripts.`,
}

var snapshotCreateCmd = &cobra.Command{
	Use:   "create <engine> <name>",
	Short: "Create a named snapshot of a database volume",
	Example: `  devx db snapshot create postgres before-migration
  devx db snapshot create redis cache-warm`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := strings.ToLower(args[0])
		name := args[1]

		if _, ok := database.Registry[engine]; !ok {
			return fmt.Errorf("unknown engine %q — supported: %s", engine, strings.Join(database.SupportedEngines(), ", "))
		}

		prov, err := getFullProvider()
		if err != nil {
			return err
		}

		meta, err := database.CreateSnapshot(prov.Runtime, engine, name)
		if err != nil {
			return err
		}

		if outputJSON {
			b, _ := json.MarshalIndent(meta, "", "  ")
			fmt.Println(string(b))
			return nil
		}

		fmt.Printf("\n✅ Snapshot created\n")
		fmt.Printf("  Name:    %s\n", meta.Name)
		fmt.Printf("  Engine:  %s\n", meta.Engine)
		fmt.Printf("  Volume:  %s\n", meta.Volume)
		fmt.Printf("  Size:    %s\n", humanizeBytes(meta.SizeBytes))
		fmt.Printf("  Saved:   %s\n", meta.CreatedAt.Format(time.RFC3339))
		fmt.Printf("\nRestore with: devx db snapshot restore %s %s\n", engine, name)
		return nil
	},
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore <engine> <name>",
	Short: "Restore a database volume from a named snapshot",
	Example: `  devx db snapshot restore postgres before-migration
  devx db snapshot restore redis cache-warm`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := strings.ToLower(args[0])
		name := args[1]

		if !NonInteractive {
			fmt.Printf("⚠️  This will stop the running %s container and overwrite its volume.\n", engine)
			fmt.Printf("   Restoring snapshot: %q\n\n", name)
			if !DryRun {
				fmt.Print("Continue? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm) //nolint:errcheck
				if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
					fmt.Println("Aborted.")
					return nil
				}
			}
		}

		if DryRun {
			fmt.Printf("[dry-run] Would stop devx-db-%s, wipe devx-data-%s, and restore snapshot %q\n", engine, engine, name)
			return nil
		}

		prov, err := getFullProvider()
		if err != nil {
			return err
		}

		return database.RestoreSnapshot(prov.Runtime, engine, name)
	},
}

var snapshotListCmd = &cobra.Command{
	Use:   "list <engine>",
	Short: "List available snapshots for a database engine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := strings.ToLower(args[0])
		snapshots, err := database.ListSnapshots(engine)
		if err != nil {
			return err
		}

		if outputJSON {
			b, _ := json.MarshalIndent(snapshots, "", "  ")
			fmt.Println(string(b))
			return nil
		}

		if len(snapshots) == 0 {
			fmt.Printf("No snapshots found for %s.\n", engine)
			fmt.Printf("Create one with: devx db snapshot create %s <name>\n", engine)
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tENGINE\tSIZE\tCREATED")
		fmt.Fprintln(w, "────\t──────\t────\t───────")
		for _, s := range snapshots {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				s.Name,
				s.Engine,
				humanizeBytes(s.SizeBytes),
				s.CreatedAt.Format("2006-01-02 15:04:05"),
			)
		}
		return w.Flush()
	},
}

var snapshotRmCmd = &cobra.Command{
	Use:   "rm <engine> <name>",
	Short: "Delete a named snapshot",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := strings.ToLower(args[0])
		name := args[1]

		if DryRun {
			fmt.Printf("[dry-run] Would delete snapshot %q for engine %s\n", name, engine)
			return nil
		}

		if err := database.DeleteSnapshot(engine, name); err != nil {
			return err
		}
		fmt.Printf("🗑️  Snapshot %q deleted.\n", name)
		return nil
	},
}

func init() {
	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotRmCmd)
	dbCmd.AddCommand(snapshotCmd)
}

// humanizeBytes converts a byte count to a human-readable string.
func humanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
