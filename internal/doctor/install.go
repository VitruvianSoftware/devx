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

package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InstallPlan describes what will be installed and the commands to run.
type InstallPlan struct {
	PackageManager string        `json:"package_manager"`
	Steps          []InstallStep `json:"steps"`
}

// InstallStep is a single install action.
type InstallStep struct {
	Tool        string `json:"tool"`
	Command     string `json:"command"`
	Tap         string `json:"tap,omitempty"`
	IsRequired  bool   `json:"required"`
	FeatureArea string `json:"feature_area,omitempty"`
}

// PlanInstall looks at missing tools and returns the commands needed to install them.
// If requiredOnly is true, only required tools are included.
func PlanInstall(requiredOnly bool) (*InstallPlan, error) {
	sys := DetectSystem()
	tools := CheckTools()

	if sys.PackageManager == "" {
		return nil, fmt.Errorf("no package manager detected — install Homebrew first: https://brew.sh")
	}

	plan := &InstallPlan{
		PackageManager: sys.PackageManager,
	}

	for _, t := range tools {
		if t.Installed {
			continue
		}
		if requiredOnly && !t.Required {
			continue
		}

		step := InstallStep{
			Tool:        t.Name,
			IsRequired:  t.Required,
			FeatureArea: t.FeatureArea,
		}

		switch sys.PackageManager {
		case "brew":
			step.Tap = t.InstallTap
			step.Command = t.InstallCmd
		case "apt":
			step.Command = mapToApt(t.Binary)
		case "dnf":
			step.Command = mapToDnf(t.Binary)
		default:
			step.Command = t.InstallCmd // fallback to brew command as hint
		}

		if step.Command != "" {
			plan.Steps = append(plan.Steps, step)
		}
	}

	return plan, nil
}

// ExecuteInstall runs the install plan, printing each command before running it.
func ExecuteInstall(plan *InstallPlan, autoConfirm bool) error {
	if len(plan.Steps) == 0 {
		return nil
	}

	// Collect all taps needed first
	taps := make(map[string]bool)
	for _, s := range plan.Steps {
		if s.Tap != "" {
			taps[s.Tap] = true
		}
	}

	// Run taps, then trust them. Newer Homebrew refuses to LOAD formulae from
	// untrusted third-party taps ("Refusing to load formula <tap>/<formula> from
	// untrusted tap <tap>. Run `brew trust <tap>` to trust it."), which is the
	// same gate that froze the brew-installed devx itself (see #434). Without the
	// trust the `brew install` steps below fail for cloudflared, mutagen, and our
	// own vitruviansoftware/tap. `brew trust` is idempotent, so re-trusting an
	// already-trusted tap is harmless.
	for tap := range taps {
		tapCmd := "brew tap " + tap
		fmt.Printf("  ⏳ %s\n", tapCmd)
		if err := runShellCommand(tapCmd); err != nil {
			return fmt.Errorf("failed to add tap %s: %w", tap, err)
		}
		fmt.Printf("  ✓  %s\n", tapCmd)

		// Best-effort: older Homebrew predates the `brew trust` subcommand, so a
		// failure here must not abort the install — warn and continue. If trust
		// was genuinely needed, the install step will surface the untrusted-tap
		// error on its own.
		trustCmd := "brew trust " + tap
		fmt.Printf("  ⏳ %s\n", trustCmd)
		if err := runShellCommand(trustCmd); err != nil {
			fmt.Printf("  ⚠  %s — %v (continuing; older Homebrew may not support `brew trust`)\n", trustCmd, err)
		} else {
			fmt.Printf("  ✓  %s\n", trustCmd)
		}
	}

	// Install each tool
	for _, s := range plan.Steps {
		fmt.Printf("  ⏳ %s\n", s.Command)
		if err := runShellCommand(s.Command); err != nil {
			fmt.Printf("  ✗  %s — %v\n", s.Command, err)
			// Continue with other installs rather than aborting
			continue
		}
		fmt.Printf("  ✓  %s\n", s.Command)
	}

	return nil
}

// runShellCommand executes a shell command, streaming its output to the
// terminal. It is a package-level variable so tests can substitute a recording
// stub for the real exec-backed runner.
var runShellCommand = func(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	c := exec.Command(parts[0], parts[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// mapToApt maps tool binary names to apt package names.
func mapToApt(binary string) string {
	m := map[string]string{
		"podman":      "sudo apt install -y podman",
		"docker":      "sudo apt install -y docker.io",
		"butane":      "sudo apt install -y butane",
		"gh":          "sudo apt install -y gh",
		"cloudflared": "", // no apt package, use deb from Cloudflare's repo
		"gcloud":      "", // special install
		"op":          "", // special install
		"bw":          "sudo apt install -y bw",
	}
	return m[binary]
}

// mapToDnf maps tool binary names to dnf package names.
func mapToDnf(binary string) string {
	m := map[string]string{
		"podman":      "sudo dnf install -y podman",
		"docker":      "sudo dnf install -y docker",
		"butane":      "sudo dnf install -y butane",
		"gh":          "sudo dnf install -y gh",
		"cloudflared": "", // no dnf package
		"gcloud":      "", // special install
		"op":          "", // special install
		"bw":          "", // special install
	}
	return m[binary]
}
