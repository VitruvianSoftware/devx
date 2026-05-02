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
