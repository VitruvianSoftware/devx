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

// Package prereqs checks and optionally installs prerequisites on remote macOS hosts.
package prereqs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// brewCmd returns the full path to brew, auto-detecting ARM64 vs Intel.
const brewDetect = `if [ -x /opt/homebrew/bin/brew ]; then echo /opt/homebrew/bin/brew; elif [ -x /usr/local/bin/brew ]; then echo /usr/local/bin/brew; else echo ""; fi`

// CheckResult holds the result of a single prerequisite check.
type CheckResult struct {
	Name      string
	Installed bool
	Message   string
}

// EnsureAll verifies and optionally installs prerequisites on a single host.
// If autoInstall is true, missing tools will be installed automatically.
func EnsureAll(ctx context.Context, node config.NodeConfig, autoInstall bool) error {
	runner := util.NewRunner(node)
	host := node.Host

	slog.Info("checking prerequisites", "host", host, "auto_install", autoInstall)
	fmt.Printf("  [%s] Checking prerequisites...\n", host)

	// 1. Detect or install Homebrew, get its full path.
	brewPath, err := detectOrInstallBrew(ctx, runner, host, autoInstall)
	if err != nil {
		return err
	}

	// 2. Check/install Lima (using full brew path).
	if err := checkOrInstallLima(ctx, runner, host, brewPath, autoInstall); err != nil {
		return err
	}

	// 3. Check/install socket_vmnet (using full brew path).
	if err := checkOrInstallSocketVmnet(ctx, runner, host, brewPath, autoInstall); err != nil {
		return err
	}

	// 4. Determine bin prefix for limactl (brew --prefix).
	binPrefix, _ := runner.RunShell(ctx, fmt.Sprintf("%s --prefix", brewPath))
	binPrefix = strings.TrimSpace(binPrefix)
	if binPrefix == "" {
		binPrefix = "/opt/homebrew" // sensible default
	}
	limactlPath := binPrefix + "/bin/limactl"

	// 5. Check sudoers for Lima.
	if err := ensureSudoers(ctx, runner, host, limactlPath); err != nil {
		return err
	}

	// 6. Ensure socket_vmnet service is running.
	if err := ensureSocketVmnetRunning(ctx, runner, host, brewPath); err != nil {
		return err
	}

	// 7. Ensure socket_vmnet directory is readable by Lima.
	_, _ = runner.RunShell(ctx, fmt.Sprintf("sudo chmod 755 $(%s --prefix)/var/run 2>/dev/null", brewPath))

	// 8. Configure Lima bridged networking.
	if err := ensureLimaBridgedNetwork(ctx, runner, host, binPrefix); err != nil {
		return err
	}

	slog.Info("all prerequisites satisfied", "host", host)
	fmt.Printf("  [%s] ✅ All prerequisites satisfied\n", host)
	return nil
}

func detectOrInstallBrew(ctx context.Context, runner *remote.Runner, host string, autoInstall bool) (string, error) {
	out, err := runner.RunShell(ctx, brewDetect)
	if err == nil {
		brewPath := strings.TrimSpace(out)
		if brewPath != "" {
			slog.Debug("homebrew found", "host", host, "path", brewPath)
			return brewPath, nil
		}
	}

	if !autoInstall {
		return "", fmt.Errorf("[%s] homebrew is not installed (use --auto-install to install automatically)", host)
	}

	slog.Info("installing homebrew", "host", host)
	fmt.Printf("  [%s] Installing Homebrew...\n", host)
	_, err = runner.RunShell(ctx, `NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)
	if err != nil {
		return "", fmt.Errorf("[%s] failed to install homebrew: %w", host, err)
	}

	// Re-detect after install.
	out, err = runner.RunShell(ctx, brewDetect)
	if err != nil {
		return "", fmt.Errorf("[%s] homebrew installed but not detected: %w", host, err)
	}
	return strings.TrimSpace(out), nil
}

func checkOrInstallLima(ctx context.Context, runner *remote.Runner, host, brewPath string, autoInstall bool) error {
	// Check using the brew prefix path for limactl.
	prefix, _ := runner.RunShell(ctx, fmt.Sprintf("%s --prefix", brewPath))
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "/opt/homebrew"
	}
	limactlPath := prefix + "/bin/limactl"

	out, err := runner.RunShell(ctx, fmt.Sprintf("%s --version 2>/dev/null", limactlPath))
	if err == nil && out != "" {
		slog.Debug("lima found", "host", host, "version", strings.TrimSpace(out))
		return nil
	}

	if !autoInstall {
		return fmt.Errorf("[%s] lima is not installed (use --auto-install to install automatically)", host)
	}

	slog.Info("installing lima", "host", host)
	fmt.Printf("  [%s] Installing Lima...\n", host)
	_, err = runner.RunShell(ctx, fmt.Sprintf("%s install lima", brewPath))
	if err != nil {
		return fmt.Errorf("[%s] failed to install lima: %w", host, err)
	}
	return nil
}

func checkOrInstallSocketVmnet(ctx context.Context, runner *remote.Runner, host, brewPath string, autoInstall bool) error {
	_, err := runner.RunShell(ctx, fmt.Sprintf("%s list socket_vmnet &>/dev/null", brewPath))
	if err == nil {
		slog.Debug("socket_vmnet found", "host", host)
		return nil
	}

	if !autoInstall {
		return fmt.Errorf("[%s] socket_vmnet is not installed (use --auto-install to install automatically)", host)
	}

	slog.Info("installing socket_vmnet", "host", host)
	fmt.Printf("  [%s] Installing socket_vmnet...\n", host)
	_, err = runner.RunShell(ctx, fmt.Sprintf("%s install socket_vmnet", brewPath))
	if err != nil {
		return fmt.Errorf("[%s] failed to install socket_vmnet: %w", host, err)
	}
	return nil
}

func ensureSudoers(ctx context.Context, runner *remote.Runner, host, limactlPath string) error {
	// Check if the sudoers file already exists.
	_, err := runner.Run(ctx, "test -f /etc/sudoers.d/lima && echo exists")
	if err == nil {
		slog.Debug("lima sudoers already configured", "host", host)
		return nil
	}

	slog.Info("configuring lima sudoers", "host", host)
	fmt.Printf("  [%s] Configuring Lima sudoers...\n", host)
	_, err = runner.RunShell(ctx, fmt.Sprintf("%s sudoers | sudo tee /etc/sudoers.d/lima >/dev/null", limactlPath))
	if err != nil {
		return fmt.Errorf("[%s] failed to configure sudoers: %w", host, err)
	}
	return nil
}

func ensureSocketVmnetRunning(ctx context.Context, runner *remote.Runner, host, brewPath string) error {
	// Auto-detect the active network interface for bridging.
	activeIface, err := runner.RunShell(ctx, "route -n get default 2>/dev/null | grep 'interface:' | sed 's/.*interface: //'")
	if err != nil || strings.TrimSpace(activeIface) == "" {
		activeIface = "en0" // fallback
	}
	activeIface = strings.TrimSpace(activeIface)
	slog.Debug("detected active network interface", "host", host, "interface", activeIface)

	// Check common socket locations (use sudo since directories may have restricted perms).
	socketPaths := []string{
		"/opt/homebrew/var/run/socket_vmnet",
		"/usr/local/var/run/socket_vmnet",
		"/var/run/socket_vmnet",
	}
	for _, p := range socketPaths {
		_, err := runner.RunShell(ctx, fmt.Sprintf("sudo test -S %s", p))
		if err == nil {
			// Check if already running in bridged mode on the correct interface.
			out, _ := runner.RunShell(ctx, "cat /Library/LaunchDaemons/homebrew.mxcl.socket_vmnet.plist 2>/dev/null")
			if strings.Contains(out, "--vmnet-mode=bridged") && strings.Contains(out, fmt.Sprintf("--vmnet-interface=%s", activeIface)) {
				slog.Debug("socket_vmnet running in bridged mode on correct interface", "host", host, "path", p, "interface", activeIface)
				return nil
			}
			// Running but wrong mode or wrong interface — need to reconfigure.
			slog.Info("reconfiguring socket_vmnet for bridged mode", "host", host, "interface", activeIface)
			break
		}
	}

	// Detect the socket_vmnet binary and brew prefix.
	prefix, _ := runner.RunShell(ctx, fmt.Sprintf("%s --prefix", brewPath))
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "/opt/homebrew"
	}

	// Find the socket_vmnet binary.
	socketBin, err := runner.RunShell(ctx, fmt.Sprintf("ls %s/Cellar/socket_vmnet/*/bin/socket_vmnet 2>/dev/null | head -1", prefix))
	if err != nil || strings.TrimSpace(socketBin) == "" {
		socketBin = prefix + "/bin/socket_vmnet"
	}
	socketBin = strings.TrimSpace(socketBin)

	socketPath := prefix + "/var/run/socket_vmnet"
	logDir := prefix + "/var/log/socket_vmnet"

	slog.Info("configuring socket_vmnet in bridged mode", "host", host, "interface", activeIface)
	fmt.Printf("  [%s] Configuring socket_vmnet bridged mode (interface=%s)...\n", host, activeIface)

	// Stop any running instance.
	_, _ = runner.RunShell(ctx, fmt.Sprintf("sudo %s services stop socket_vmnet 2>/dev/null", brewPath))
	_, _ = runner.RunShell(ctx, "sudo launchctl bootout system /Library/LaunchDaemons/homebrew.mxcl.socket_vmnet.plist 2>/dev/null")
	_, _ = runner.RunShell(ctx, "sudo launchctl bootout system/homebrew.mxcl.socket_vmnet 2>/dev/null")
	_, _ = runner.RunShell(ctx, "sleep 1")

	// Create a custom plist for bridged mode.
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>homebrew.mxcl.socket_vmnet</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>--vmnet-mode=bridged</string>
    <string>--vmnet-interface=%s</string>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardErrorPath</key>
  <string>%s/stderr</string>
  <key>StandardOutPath</key>
  <string>%s/stdout</string>
</dict>
</plist>`, socketBin, activeIface, socketPath, logDir, logDir)

	encoded := util.Base64Encode(plist)
	_, err = runner.RunShell(ctx, fmt.Sprintf("sudo mkdir -p %s && echo %s | base64 -d | sudo tee /Library/LaunchDaemons/homebrew.mxcl.socket_vmnet.plist > /dev/null", logDir, encoded))
	if err != nil {
		return fmt.Errorf("[%s] failed to write socket_vmnet plist: %w", host, err)
	}

	// Make sure the socket directory is accessible.
	_, _ = runner.RunShell(ctx, fmt.Sprintf("sudo mkdir -p $(dirname %s) && sudo chmod 755 $(dirname %s)", socketPath, socketPath))

	// Start the service using our custom plist exclusively.
	_, _ = runner.RunShell(ctx, "sudo launchctl enable system/homebrew.mxcl.socket_vmnet 2>/dev/null")
	_, err = runner.RunShell(ctx, "sudo launchctl bootstrap system /Library/LaunchDaemons/homebrew.mxcl.socket_vmnet.plist")
	if err != nil {
		return fmt.Errorf("[%s] failed to start socket_vmnet bridged service: %w", host, err)
	}

	// Wait briefly for socket to appear.
	_, _ = runner.RunShell(ctx, "sleep 2")
	_, err = runner.RunShell(ctx, fmt.Sprintf("sudo test -S %s", socketPath))
	if err != nil {
		return fmt.Errorf("[%s] socket_vmnet started but socket not found at %s", host, socketPath)
	}

	slog.Info("socket_vmnet running in bridged mode", "host", host, "interface", activeIface)
	return nil
}

func ensureLimaBridgedNetwork(_ context.Context, _ *remote.Runner, _ string, _ string) error {
	// No longer needed — bridged mode is configured at the socket_vmnet service level.
	return nil
}
