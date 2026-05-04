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
	"os/exec"

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

// ollamaLaunchable maps devx agent IDs to ollama launch integration names.
// Only agents supported by `ollama launch` are listed here.
var ollamaLaunchable = map[string]string{
	"claude": "claude",
	// "codex":    "codex",   // uncomment when we add codex to the agent selector
	// "opencode": "opencode", // uncomment when we add opencode to the agent selector
}

var agentInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AI Agent tools/manifests for the devx environment",
	Long: `Automatically configures standard AI agent constraints (.cursorrules, CLAUDE.md, etc)
so agents understand devx conventions like --json and --dry-run.

If Ollama is installed, you will also be offered the option to configure
selected coding agents to use local models via 'ollama launch --config'.
This eliminates the need for cloud API keys.`,
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
					_, _ = fmt.Fprintf(os.Stderr, "failed to install %s/%s config: %v\n", a, s, err)
				}
			}
		}

		fmt.Println("\n✅ AI Agent manifests are ready!")

		// ── Step 3: Offer ollama launch --config ─────────────────────
		offerOllamaLaunch(selectedAgents)

		return nil
	},
}

// offerOllamaLaunch checks if Ollama is installed and any selected agents
// support `ollama launch`. If so, it offers to auto-configure them to use
// local models — no API keys needed.
func offerOllamaLaunch(selectedAgents []string) {
	// Is ollama even installed?
	ollamaPath, err := exec.LookPath("ollama")
	if err != nil {
		return // Ollama not installed, nothing to offer
	}

	// Which selected agents support ollama launch?
	var launchable []string
	for _, a := range selectedAgents {
		if ollamaName, ok := ollamaLaunchable[a]; ok {
			// Check if this agent's binary is actually installed
			if _, err := exec.LookPath(ollamaName); err == nil {
				launchable = append(launchable, ollamaName)
			}
		}
	}

	if len(launchable) == 0 {
		return
	}

	if NonInteractive {
		// In non-interactive mode, just print a hint
		for _, name := range launchable {
			fmt.Printf("\n💡 Tip: Run 'ollama launch %s' to connect %s to local models.\n", name, name)
		}
		return
	}

	// Build options for the ollama launch prompt
	var ollamaOptions []huh.Option[string]
	for _, name := range launchable {
		label := fmt.Sprintf("Configure %s to use local Ollama models", name)
		ollamaOptions = append(ollamaOptions, huh.NewOption(label, name).Selected(true))
	}

	var selectedLaunch []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Connect agents to local LLM via Ollama?").
				Description("This runs 'ollama launch --config' — no cloud API keys needed.").
				Options(ollamaOptions...).
				Value(&selectedLaunch),
		),
	).WithTheme(huh.ThemeCatppuccin())

	if err := form.Run(); err != nil || len(selectedLaunch) == 0 {
		return
	}

	fmt.Println("\n🔗 Configuring local LLM connections...")
	for _, name := range selectedLaunch {
		fmt.Printf("  → ollama launch %s --config\n", name)
		if DryRun {
			fmt.Printf("  [dry-run] Would run: %s launch %s --config\n", ollamaPath, name)
			continue
		}
		launchCmd := exec.Command(ollamaPath, "launch", name, "--config")
		launchCmd.Stdout = os.Stdout
		launchCmd.Stderr = os.Stderr
		launchCmd.Stdin = os.Stdin
		if err := launchCmd.Run(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "  ⚠️  ollama launch %s failed: %v\n", name, err)
		} else {
			fmt.Printf("  ✓ %s configured for local models\n", name)
		}
	}
}

func init() {
	agentInitCmd.Flags().BoolVarP(&agentForceUpdate, "force", "f", false, "Force overwrite existing AI agent manifests")

	agentCmd.AddCommand(agentInitCmd)
	rootCmd.AddCommand(agentCmd)
}

