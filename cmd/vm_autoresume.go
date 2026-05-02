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
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/secrets"
)

// ensureVMRunning is a lifecycle hook invoked by devx CLI commands
// that require the local hypervisor/provider to be un-suspended.
func ensureVMRunning() error {
	if DryRun {
		return nil
	}

	prov, err := getVMProvider()
	if err != nil {
		return err
	}

	// Docker Desktop and OrbStack manage their VM sleep state natively
	// via macOS. All other providers (podman, lima, colima) need explicit
	// wake-up via Start().
	if prov.Name() == "docker" {
		return nil
	}

	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, "", "", "")
	if s, err := secrets.Load(envFile); err == nil && s.DevHostname != "" {
		cfg.DevHostname = s.DevHostname
	}
	if cfg.DevHostname == "" {
		cfg.DevHostname = "devx"
	}

	if !prov.IsRunning(cfg.DevHostname) {
		fmt.Fprintln(os.Stderr, "😴 VM is asleep. Waking it up...")
		if err := prov.Start(cfg.DevHostname); err != nil {
			if strings.Contains(err.Error(), "VM does not exist") {
				var setup bool
				pErr := huh.NewConfirm().
					Title("VM does not exist. Would you like to initialize it now?").
					Value(&setup).
					Run()

				if pErr == nil && setup {
					if initErr := runInit(nil, nil); initErr != nil {
						return fmt.Errorf("initialization failed: %w", initErr)
					}
					return ensureVMRunning()
				}

				return devxerr.New(devxerr.CodeVMNotFound, "VM does not exist. Run 'devx vm init' to create it.", err)
			}
			return devxerr.New(devxerr.CodeVMDormant, "Failed to wake up VM", err)
		}
	}

	return nil
}
