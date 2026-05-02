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
	Short: "Hard reset: purge caches, builds, and devx resources for this project",
	Long: `Scans the current project directory for language-specific caches, build
artefacts, and devx-managed containers/volumes.

Presents an interactive multi-select so you can choose exactly what to delete
while leaving things that are still working untouched.

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

	prov, err := getFullProvider()
	if err != nil {
		return err
	}

	// Collect the manifest — nothing is deleted yet
	manifest, err := nuke.Collect(cwd, prov.Runtime)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(manifest.Items) == 0 {
		fmt.Printf("%s Nothing to nuke — your project is already clean!\n", tui.IconDone)
		return nil
	}

	// Print the "safe" zone header once so the user has context.
	printSafeZone()

	if DryRun {
		printNukeManifest(manifest, manifest.Items)
		fmt.Printf("[dry-run] Would delete %d items (%s total).\n",
			len(manifest.Items), nuke.FormatBytes(manifest.TotalSize))
		return nil
	}

	// ── Phase 1: multi-select ────────────────────────────────────────────────
	// Build all options, pre-selected by default.
	var selectedIndices []int
	if !NonInteractive {
		options := make([]huh.Option[int], len(manifest.Items))
		for i, item := range manifest.Items {
			label := nukeOptionLabel(item)
			options[i] = huh.NewOption(label, i).Selected(true)
		}

		multiSelect := huh.NewMultiSelect[int]().
			Title("Select items to nuke  (Space to toggle, Enter to confirm)").
			Description("All items are pre-selected. Deselect anything you want to keep.").
			Options(options...).
			Value(&selectedIndices)

		if err := huh.NewForm(huh.NewGroup(multiSelect)).
			WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			fmt.Println("\nCancelled — nothing was deleted.")
			return nil
		}

		if len(selectedIndices) == 0 {
			fmt.Println("\nNothing selected — nothing was deleted.")
			return nil
		}
	} else {
		// -y: select all
		for i := range manifest.Items {
			selectedIndices = append(selectedIndices, i)
		}
	}

	// Build the filtered item list from selected indices
	selected := make([]nuke.Item, 0, len(selectedIndices))
	var selectedSize int64
	for _, idx := range selectedIndices {
		selected = append(selected, manifest.Items[idx])
		selectedSize += manifest.Items[idx].SizeBytes
	}

	// ── Phase 2: final confirmation ──────────────────────────────────────────
	fmt.Println()
	printNukeManifest(manifest, selected)

	if !NonInteractive {
		var confirmed bool
		dangerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true)
		confirmTitle := dangerStyle.Render(fmt.Sprintf("⚠  Delete %d item(s) (%s)?  This cannot be undone.",
			len(selected), nuke.FormatBytes(selectedSize)))

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(confirmTitle).
					Affirmative("Yes, nuke selected").
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

	// ── Phase 3: execute ─────────────────────────────────────────────────────
	fmt.Println(tui.StyleTitle.Render("Nuking...") + "\n")
	errCount := 0

	// Execute only the selected subset
	selectedManifest := &nuke.Manifest{
		Items:     selected,
		TotalSize: selectedSize,
		Runtime:   manifest.Runtime,
	}
	selectedManifest.Execute(func(item nuke.Item, err error) {
		icon := tui.IconDone
		detail := ""
		if err != nil {
			icon = tui.IconFailed
			detail = tui.StyleDetailError.Render(fmt.Sprintf("  error: %v", err))
			errCount++
		}
		category := tui.StyleMuted.Render(fmt.Sprintf("%-10s", item.Category))
		label := tui.StyleValue.Render(item.Label)
		size := tui.StyleMuted.Render(item.SizeDisplay)
		fmt.Printf("  %s  %s  %s  %s%s\n", icon, category, label, size, detail)
	})

	fmt.Println()
	if errCount > 0 {
		fmt.Printf("%s %d item(s) could not be deleted (see above).\n", tui.IconFailed, errCount)
		fmt.Println("  Some items may require sudo — run: sudo devx nuke")
	} else {
		fmt.Printf("%s Nuked %d item(s) (%s freed).\n",
			tui.IconDone, len(selected), nuke.FormatBytes(selectedSize))
		fmt.Printf("\n  Run %s to get a fresh environment.\n",
			tui.StyleURL.Render("devx up"))
	}

	return nil
}

// nukeOptionLabel builds the multi-select display string for a single item.
// Format: [Category] label  (size)
func nukeOptionLabel(item nuke.Item) string {
	catStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF"))
	sizeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))
	cat := catStyle.Render(fmt.Sprintf("[%-8s]", item.Category))
	size := sizeStyle.Render(fmt.Sprintf("(%s)", item.SizeDisplay))
	return fmt.Sprintf("%s  %-42s  %s", cat, item.Label, size)
}

// printNukeManifest displays the filtered selection grouped by category.
func printNukeManifest(manifest *nuke.Manifest, selected []nuke.Item) {
	// Group selected items by category
	categoryOrder := []string{}
	byCategory := map[string][]nuke.Item{}
	for _, item := range selected {
		if _, exists := byCategory[item.Category]; !exists {
			categoryOrder = append(categoryOrder, item.Category)
		}
		byCategory[item.Category] = append(byCategory[item.Category], item)
	}

	_ = manifest // reserved for future context
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341")).Bold(true)

	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).
		Render("  Selected for deletion:\n"))

	var totalSize int64
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
			totalSize += item.SizeBytes
		}
		fmt.Println()
	}

	boldYellow := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E3B341"))
	if totalSize > 0 {
		fmt.Printf("  %s\n\n", boldYellow.Render(
			fmt.Sprintf("Total: %s across %d item(s)", nuke.FormatBytes(totalSize), len(selected)),
		))
	} else {
		fmt.Printf("  %s\n\n", tui.StyleMuted.Render(
			fmt.Sprintf("%d item(s) (containers and volumes)", len(selected)),
		))
	}
}

// printSafeZone prints the items that are never touched, for developer peace of mind.
func printSafeZone() {
	safeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))
	fmt.Println(safeStyle.Render("  Safe (never touched):"))
	for _, safe := range []string{"Source code", ".env files", "devx.yaml", "SSH keys", "~/.devx/snapshots"} {
		fmt.Printf("    %s  %s\n", tui.IconDone, safe)
	}
	fmt.Println()
}
