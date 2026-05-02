// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
	GroupID: "ci",
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
		var selectedSkills []string

		if len(args) > 0 {
			selectedAgents = args
			// Non-interactive fallback: assume all available skills if args are passed
			for _, skill := range agent.AvailableSkills {
				selectedSkills = append(selectedSkills, skill.ID)
			}
		} else if NonInteractive {
			// In non-interactive mode without args, do nothing
			return nil
		} else {
			var skillOptions []huh.Option[string]
			for _, skill := range agent.AvailableSkills {
				label := fmt.Sprintf("%s — %s", skill.Name, skill.Description)
				skillOptions = append(skillOptions, huh.NewOption(label, skill.ID).Selected(true))
			}

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Which AI Agent(s) do you use?").
						Description("We will initialize the correct directory structures for them.").
						Options(
							huh.NewOption("Antigravity/Gemini (Standard Agent Skills)", "antigravity").Selected(true),
							huh.NewOption("Cursor IDE", "cursor"),
							huh.NewOption("Claude Code (Anthropic)", "claude"),
							huh.NewOption("GitHub Copilot Chat", "copilot"),
						).
						Value(&selectedAgents),
				),
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Which skills should we inject?").
						Description("Select standard operating procedures to enforce for this repo.").
						Options(skillOptions...).
						Value(&selectedSkills),
				),
			).WithTheme(huh.ThemeCatppuccin())

			if err := form.Run(); err != nil {
				fmt.Println("Agent initialization cancelled.")
				return nil
			}
		}

		if len(selectedAgents) == 0 || len(selectedSkills) == 0 {
			fmt.Println("No agents or skills selected. Doing nothing.")
			return nil
		}

		fmt.Println("📦 Installing devx configurations...")
		for _, a := range selectedAgents {
			for _, s := range selectedSkills {
				if err := agent.Install(a, s, agentForceUpdate); err != nil {
					fmt.Fprintf(os.Stderr, "failed to install %s/%s config: %v\n", a, s, err)
				}
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
