package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/charmbracelet/huh"
)

// ensureCloudflareLogin checks if cloudflared is authenticated.
// If not, it interactively prompts the user to login right then and there.
func ensureCloudflareLogin() error {
	if err := cloudflare.CheckLogin(); err == nil {
		return nil
	}

	var login bool
	pErr := huh.NewConfirm().
		Title("Cloudflare credentials missing. Would you like to login to Cloudflare now?").
		Value(&login).
		Run()

	if pErr == nil && login {
		fmt.Println("Opening browser to authenticate with Cloudflare...")
		cmd := exec.Command("cloudflared", "tunnel", "login")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Cloudflare login failed: %w", err)
		}
		// Confirm login succeeded
		if err := cloudflare.CheckLogin(); err == nil {
			return nil
		}
	}

	return devxerr.New(devxerr.CodeNotLoggedIn, "Cloudflare credentials missing. Run 'cloudflared tunnel login'", nil)
}
