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
	"github.com/VitruvianSoftware/devx/internal/prereqs"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/VitruvianSoftware/devx/internal/tailscale"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

const butaneTemplatePath = "dev-machine.template.bu"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Provision the local dev environment",
	Long:  `Full first-time setup: Cloudflare tunnel, VM, Tailscale.`,
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	// ── Resolve the virtualization provider ──────────────────────────────────
	vm, err := getVMProvider()
	if err != nil {
		return err
	}

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
		fmt.Sprintf("Prepare %s VM", vm.Name()),
		"Provision VM",
		"Start VM",
		"Deploy Configuration via SSH",
		"Wait for Tailscale daemon",
		"Authenticate Tailscale",
	}

	// Create an SSH function that delegates to the resolved provider
	sshFn := func(machine, command string) (string, error) {
		return vm.SSH(machine, command)
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
			_ = vm.StopAll()
			_ = vm.Remove(cfg.DevHostname)
			return "stopped and removed " + cfg.DevHostname, nil
		},
		func() (string, error) {
			err := vm.Init(cfg.DevHostname)
			if err != nil {
				return "", err
			}
			return cfg.DevHostname + " initialized (" + vm.Name() + ")", nil
		},
		func() (string, error) {
			if err := vm.Start(cfg.DevHostname); err != nil {
				return "", err
			}
			if err := vm.SetDefault(cfg.DevHostname); err != nil {
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
			if err := ignition.DeployWithSSH(state.ignPath, cfg.DevHostname, sshFn); err != nil {
				return "", err
			}
			return "ignition config pushed via SSH", nil
		},
		func() (string, error) {
			if err := tailscale.WaitForDaemonWithSSH(cfg.DevHostname, 3*time.Minute, sshFn); err != nil {
				return "", err
			}
			return "tailscaled container running", nil
		},
		func() (string, error) {
			authURL, err := tailscale.UpWithSSH(cfg.DevHostname, cfg.DevHostname, sshFn)
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
		_, _ = fmt.Scanln()
	}

	if !sm.IsDone() || sm.IsFailed() {
		return fmt.Errorf("setup did not complete successfully")
	}

	// ── Success summary ───────────────────────────────────────────────────────
	fmt.Printf("\n🎉 Setup complete! (provider: %s)\n\n", vm.Name())
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
