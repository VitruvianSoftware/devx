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

package lima

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// NodeKeeperLabel is the launchd label for the per-host VM keeper LaunchAgent.
const NodeKeeperLabel = "io.devx.node-vm-keeper"

// nodeKeeperScript renders the keeper script. It waits (bounded) for the
// socket_vmnet socket — whose launchd KeepAlive heals it after a reboot — then
// starts the Lima node VM only if it is not already Running. Idempotent, so it is
// safe to run at login and on an interval.
func nodeKeeperScript(limactlPath, vmName, socketPath string) string {
	return fmt.Sprintf(`#!/bin/bash
# devx node-vm-keeper — GENERATED, do not edit by hand.
# Keeps the Lima k3s node VM running across host reboots/sleep. socket_vmnet's
# own launchd KeepAlive recovers the bridge; this only waits for its socket, then
# starts the VM if it is not already running. Idempotent.
set -u
LIMACTL=%q
VM=%q
SOCK=%q
for _ in $(seq 1 30); do [ -S "$SOCK" ] && break; sleep 2; done
status="$("$LIMACTL" list "$VM" --format '{{.Status}}' 2>/dev/null)"
if [ "$status" != "Running" ]; then
  echo "$(date '+%%Y-%%m-%%dT%%H:%%M:%%S') devx node-keeper: VM $VM is '${status:-absent}', starting"
  "$LIMACTL" start "$VM" --tty=false 2>&1 | tail -3
fi
`, limactlPath, vmName, socketPath)
}

// nodeKeeperPlist renders the LaunchAgent that runs the keeper script at GUI
// login (RunAtLoad) and every 120s (a watchdog that also recovers a VM killed by
// sleep/wake). It must be a LaunchAgent, not a LaunchDaemon: the vz driver needs
// the user's GUI session, which a root daemon does not have.
func nodeKeeperPlist(scriptPath, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>StartInterval</key>
  <integer>120</integer>
  <key>StandardErrorPath</key>
  <string>%s</string>
  <key>StandardOutPath</key>
  <string>%s</string>
</dict>
</plist>`, NodeKeeperLabel, scriptPath, logPath, logPath)
}

// InstallNodeKeeper installs the node-vm-keeper LaunchAgent on the remote Mac
// host so the Lima node VM self-recovers after a host reboot or sleep. It writes
// the keeper script and the LaunchAgent plist into the user's home (the plist
// auto-loads at the next GUI login; no launchctl bootstrap over SSH is required,
// which would fail without an Aqua session). Best-effort and idempotent.
func (m *Manager) InstallNodeKeeper(ctx context.Context) error {
	socketPath, err := m.detectSocketPath(ctx)
	if err != nil {
		return fmt.Errorf("detecting socket_vmnet path for keeper: %w", err)
	}
	home, err := m.runner.Run(ctx, "echo $HOME")
	if err != nil {
		return fmt.Errorf("resolving home dir: %w", err)
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return fmt.Errorf("[%s] empty home dir", m.runner.Host)
	}

	// launchd exec's ProgramArguments directly (no shell, no ~ expansion), so the
	// limactl path must be absolute.
	limactlPath, err := m.runner.Run(ctx, "command -v limactl")
	limactlPath = strings.TrimSpace(limactlPath)
	if err != nil || limactlPath == "" {
		limactlPath = "/opt/homebrew/bin/limactl"
	}

	scriptPath := home + "/.devx/node-vm-keeper.sh"
	plistPath := home + "/Library/LaunchAgents/" + NodeKeeperLabel + ".plist"
	logPath := home + "/.devx/node-vm-keeper.log"

	script := util.Base64Encode(nodeKeeperScript(limactlPath, m.vmName, socketPath))
	plist := util.Base64Encode(nodeKeeperPlist(scriptPath, logPath))

	if _, err := m.runner.Run(ctx, fmt.Sprintf(
		"mkdir -p %s/.devx %s/Library/LaunchAgents && echo %s | base64 -d > %s && chmod +x %s",
		home, home, script, scriptPath, scriptPath)); err != nil {
		return fmt.Errorf("writing keeper script: %w", err)
	}
	if _, err := m.runner.Run(ctx, fmt.Sprintf("echo %s | base64 -d > %s", plist, plistPath)); err != nil {
		return fmt.Errorf("writing keeper LaunchAgent: %w", err)
	}

	// Best-effort: activate it now if a GUI session exists. Harmless if it does
	// not (the plist still auto-loads at the next login).
	uidOut, _ := m.runner.Run(ctx, "id -u")
	if uid := strings.TrimSpace(uidOut); uid != "" {
		_, _ = m.runner.Run(ctx, fmt.Sprintf("launchctl bootstrap gui/%s %s 2>/dev/null || true", uid, plistPath))
	}

	slog.Info("installed node-vm-keeper LaunchAgent", "host", m.runner.Host, "vm", m.vmName)
	return nil
}
