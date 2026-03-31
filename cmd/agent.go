package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/agent"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage AI Agent configuration for your project",
}

var (
	agentForceUpdate bool
)

var agentInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AI Agent tools/manifests for the devx environment",
	Long:  `Automatically configures standard AI agent constraints (.cursorrules, CLAUDE.md, etc) so agents understand devx conventions like --json and --dry-run.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var selectedAgents []string
		
		if len(args) > 0 {
			selectedAgents = args
		} else if NonInteractive {
			// In non-interactive mode without args, do nothing
			return nil
		} else {
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Which AI Agent(s) do you use?").
						Description("We will initialize the correct devx configuration files for them.").
						Options(
							huh.NewOption("Antigravity/Gemini (Standard Agent Skills)", "antigravity").Selected(true),
							huh.NewOption("Cursor IDE", "cursor"),
							huh.NewOption("Claude Code (Anthropic)", "claude"),
							huh.NewOption("GitHub Copilot Chat", "copilot"),
						).
						Value(&selectedAgents),
				),
			).WithTheme(huh.ThemeCatppuccin())

			if err := form.Run(); err != nil {
				fmt.Println("Agent initialization cancelled.")
				return nil
			}
		}

		if len(selectedAgents) == 0 {
			fmt.Println("No agents selected. Doing nothing.")
			return nil
		}

		fmt.Println("📦 Installing devx configurations...")
		for _, a := range selectedAgents {
			if err := agent.Install(a, agentForceUpdate); err != nil {
				fmt.Fprintf(os.Stderr, "failed to install %s config: %v\n", a, err)
			}
		}

		fmt.Println("\n✅ AI Agent manifests are ready!")
		return nil
	},
}

func init() {
	agentInitCmd.Flags().BoolVarP(&agentForceUpdate, "force", "f", false, "Force overwrite existing AI agent manifests")

	agentCmd.AddCommand(agentInitCmd)
	rootCmd.AddCommand(agentCmd)
}
