package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/ignition"
	"github.com/VitruvianSoftware/devx/internal/podman"
	"github.com/VitruvianSoftware/devx/internal/prereqs"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/VitruvianSoftware/devx/internal/tailscale"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

const butaneTemplatePath = "dev-machine.template.bu"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Provision the local dev environment",
	Long:  `Full first-time setup: Cloudflare tunnel, Podman VM, Tailscale.`,
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	// ── Load secrets (prompt via huh if .env is missing/incomplete) ──────────
	s, err := secrets.Load(envFile)
	if err != nil {
		return fmt.Errorf("secrets: %w", err)
	}

	// ── Build config from secrets + defaults ──────────────────────────────────
	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, s.DevHostname, "", "")

	// ── Shared mutable state across step closures ─────────────────────────────
	state := &setupState{cfg: cfg, token: s.CFTunnelToken}

	// ── Step definitions (name + closure) ─────────────────────────────────────
	names := []string{
		"Check prerequisites",
		"Verify Cloudflare login",
		"Set up Cloudflare tunnel",
		"Route DNS",
		"Build Ignition config",
		"Prepare Podman machine",
		"Provision VM",
		"Start VM",
		"Deploy Configuration via SSH",
		"Wait for Tailscale daemon",
		"Authenticate Tailscale",
	}

	fns := []func() (string, error){
		func() (string, error) {
			results := prereqs.Check()
			if !prereqs.AllPassed(results) {
				missing := prereqs.MissingList(results)
				return "", fmt.Errorf("missing tools: %v\nInstall: brew install podman cloudflared butane", missing)
			}
			return prereqs.Summary(results), nil
		},
		func() (string, error) {
			return "authenticated", cloudflare.CheckLogin()
		},
		func() (string, error) {
			tunnel, err := cloudflare.EnsureTunnel(cfg.TunnelName)
			if err != nil {
				return "", err
			}
			state.tunnelID = tunnel.ID

			// Read the credentials file
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			credsBytes, err := os.ReadFile(home + "/.cloudflared/" + tunnel.ID + ".json")
			if err != nil {
				return "", fmt.Errorf("reading tunnel credentials: %w", err)
			}
			state.token = string(credsBytes) // repurpose token field for credsJSON

			return tunnel.Name + " (" + tunnel.ID[:8] + "...)", nil
		},
		func() (string, error) {
			err := cloudflare.RouteDNS(cfg.TunnelName, cfg.CFDomain)
			if err != nil {
				return "", err
			}
			return cfg.CFDomain + " → tunnel", nil
		},
		func() (string, error) {
			path, err := ignition.Build(butaneTemplatePath, state.token, state.tunnelID, cfg.DevHostname, cfg.CFDomain)
			if err != nil {
				return "", err
			}
			state.ignPath = path
			return "compiled from " + butaneTemplatePath, nil
		},
		func() (string, error) {
			podman.StopAll()
			podman.Remove(cfg.DevHostname)
			return "stopped and removed " + cfg.DevHostname, nil
		},
		func() (string, error) {
			err := podman.Init(cfg.DevHostname)
			if err != nil {
				return "", err
			}
			return cfg.DevHostname + " initialized", nil
		},
		func() (string, error) {
			if err := podman.Start(cfg.DevHostname); err != nil {
				return "", err
			}
			if err := podman.SetDefault(cfg.DevHostname); err != nil {
				return "", err
			}
			return "running, set as default connection", nil
		},
		func() (string, error) {
			defer func() {
				if state.ignPath != "" {
					os.Remove(state.ignPath)
					state.ignPath = ""
				}
			}()
			if err := ignition.Deploy(state.ignPath, cfg.DevHostname); err != nil {
				return "", err
			}
			return "ignition config pushed via SSH", nil
		},
		func() (string, error) {
			if err := tailscale.WaitForDaemon(cfg.DevHostname, 3*time.Minute); err != nil {
				return "", err
			}
			return "tailscaled container running", nil
		},
		func() (string, error) {
			authURL, err := tailscale.Up(cfg.DevHostname, cfg.DevHostname)
			if err != nil {
				return "", err
			}
			state.authURL = authURL
			if authURL != "" {
				return "waiting for browser auth — see URL above", nil
			}
			return "authenticated", nil
		},
	}

	// ── Launch TUI ────────────────────────────────────────────────────────────
	m := tui.NewSetupModel(names, fns)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Print auth URL to stdout if Tailscale needed it (alt-screen hides it)
	sm := finalModel.(tui.SetupModel)
	if state.authURL != "" {
		fmt.Println()
		fmt.Println("🔗 Tailscale auth URL:")
		fmt.Println("  ", state.authURL)
		fmt.Println()
		fmt.Println("Visit the URL above to connect to your Tailnet, then press Enter...")
		fmt.Scanln()
	}

	if !sm.IsDone() || sm.IsFailed() {
		return fmt.Errorf("setup did not complete successfully")
	}

	// ── Success summary ───────────────────────────────────────────────────────
	fmt.Printf("\n🎉 Setup complete!\n\n")
	fmt.Printf("  Public endpoint:  https://%s → :8080\n", cfg.CFDomain)
	fmt.Printf("  VM hostname:      %s\n", cfg.DevHostname)
	fmt.Printf("  Tailnet:          connected\n\n")
	fmt.Printf("  Quick start:      devx exec podman run -d -p 8080:80 docker.io/nginx\n")
	fmt.Printf("  Then visit:       https://%s\n\n", cfg.CFDomain)
	fmt.Printf("  Troubleshoot:\n")
	fmt.Printf("    devx vm ssh            — SSH into VM\n")
	fmt.Printf("    devx vm status         — Check service health\n")

	return nil
}

// setupState holds mutable values shared across step closures.
type setupState struct {
	cfg      *config.Config
	token    string
	tunnelID string
	ignPath  string
	authURL  string
}

func init() {
	vmCmd.AddCommand(initCmd)
}
