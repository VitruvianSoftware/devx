package cmd

import (
	"fmt"
	"os"

	"github.com/VitruvianSoftware/devx/internal/nuke"
	"github.com/VitruvianSoftware/devx/internal/tui"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var nukeRuntime string

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Hard reset: purge all caches, builds, and devx resources for this project",
	Long: `Scans the current project directory for language-specific caches and build
artefacts, lists all devx-managed databases and containers, shows you exactly
what will be deleted with disk sizes, and asks for confirmation before touching
anything.

Does NOT delete your source code, .env files, or devx.yaml.

After nuking, run 'devx up' to get a 100% fresh environment.`,
	RunE: runNuke,
}

func init() {
	nukeCmd.Flags().StringVar(&nukeRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	rootCmd.AddCommand(nukeCmd)
}

func runNuke(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine current directory: %w", err)
	}

	fmt.Printf("%s\n\n", tui.StyleTitle.Render("devx nuke — scanning project..."))

	// Collect the manifest — nothing is deleted yet
	manifest, err := nuke.Collect(cwd, nukeRuntime)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(manifest.Items) == 0 {
		fmt.Println("✓ Nothing to nuke — your project is already clean!")
		return nil
	}

	// Print the manifest grouped by category
	printNukeManifest(manifest)

	if DryRun {
		fmt.Printf("\n[dry-run] Would delete %d items (%s total).\n",
			len(manifest.Items), nuke.FormatBytes(manifest.TotalSize))
		return nil
	}

	// Confirmation
	if !NonInteractive {
		var confirmed bool
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true)
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(warningStyle.Render("⚠ This cannot be undone.")).
					Description(fmt.Sprintf(
						"Delete %d items (%s) from your project?\n\nDatabases, containers, caches, and build artefacts will be permanently removed.\nYour source code and config files (.env, devx.yaml) are safe.",
						len(manifest.Items), nuke.FormatBytes(manifest.TotalSize),
					)).
					Affirmative("Yes, nuke it all").
					Negative("Cancel").
					Value(&confirmed),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("\nCancelled — nothing was deleted.")
			return nil
		}
		fmt.Println()
	}

	// Execute deletions, reporting progress for each item
	fmt.Println(tui.StyleTitle.Render("Nuking...") + "\n")
	errors := 0
	manifest.Execute(func(item nuke.Item, err error) {
		icon := tui.IconDone
		detail := ""
		if err != nil {
			icon = tui.IconFailed
			detail = tui.StyleDetailError.Render(fmt.Sprintf("  error: %v", err))
			errors++
		}
		category := tui.StyleMuted.Render(fmt.Sprintf("%-10s", item.Category))
		label := tui.StyleValue.Render(item.Label)
		size := tui.StyleMuted.Render(item.SizeDisplay)
		fmt.Printf("  %s  %s  %s  %s%s\n", icon, category, label, size, detail)
	})

	fmt.Println()
	if errors > 0 {
		fmt.Printf("%s %d items could not be deleted (see above).\n",
			tui.IconFailed, errors)
		fmt.Println("  Some items may require sudo — run: sudo devx nuke")
	} else {
		deletedCount := len(manifest.Items)
		fmt.Printf("%s Nuked %d items (%s freed).\n",
			tui.IconDone,
			deletedCount,
			nuke.FormatBytes(manifest.TotalSize),
		)
		fmt.Printf("\n  Run %s to get a fresh environment.\n",
			tui.StyleURL.Render("devx up"))
	}

	return nil
}

// printNukeManifest displays the grouped manifest with sizes before confirmation.
func printNukeManifest(manifest *nuke.Manifest) {
	// Group by category
	categoryOrder := []string{}
	byCategory := map[string][]nuke.Item{}
	for _, item := range manifest.Items {
		if _, exists := byCategory[item.Category]; !exists {
			categoryOrder = append(categoryOrder, item.Category)
		}
		byCategory[item.Category] = append(byCategory[item.Category], item)
	}

	dangerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341")).Bold(true)

	fmt.Println(dangerStyle.Render("  The following will be permanently deleted:\n"))

	for _, cat := range categoryOrder {
		items := byCategory[cat]
		fmt.Printf("  %s\n", headerStyle.Render(cat))
		for _, item := range items {
			sizeStr := tui.StyleMuted.Render(fmt.Sprintf("(%s)", item.SizeDisplay))
			fmt.Printf("    %s  %-45s  %s\n",
				tui.StyleDetailError.Render("✗"),
				item.Label,
				sizeStr,
			)
			if item.Path != "" {
				fmt.Printf("       %s\n", tui.StyleMuted.Render(item.Path))
			}
		}
		fmt.Println()
	}

	totalStr := nuke.FormatBytes(manifest.TotalSize)
	if manifest.TotalSize > 0 {
		fmt.Printf("  %s\n\n",
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E3B341")).
				Render(fmt.Sprintf("Total: %s across %d items", totalStr, len(manifest.Items))),
		)
	} else {
		fmt.Printf("  %s\n\n",
			tui.StyleMuted.Render(fmt.Sprintf("%d items (containers and volumes)", len(manifest.Items))),
		)
	}

	// Explicitly call out what is NOT touched
	safeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))
	fmt.Println(safeStyle.Render("  Safe (never touched):"))
	for _, safe := range []string{"Source code", ".env files", "devx.yaml", "SSH keys", "~/.devx/snapshots"} {
		fmt.Printf("    %s  %s\n", tui.IconDone, safe)
	}
	fmt.Println()
}
