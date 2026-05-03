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

// Package lima handles generating Lima configuration files and managing
// VM lifecycle (create, start, stop, delete, status) on remote hosts.
package lima

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// VMStatus represents the state of a Lima VM.
type VMStatus string

const (
	VMStatusRunning    VMStatus = "Running"
	VMStatusStopped    VMStatus = "Stopped"
	VMStatusNotCreated VMStatus = "NotCreated"
	VMStatusUnknown    VMStatus = "Unknown"
)

// Manager handles Lima VM operations on a remote host.
type Manager struct {
	runner *remote.Runner
	node   config.NodeConfig
	vmName string
}

// NewManager creates a new Lima manager for the given host and node config.
func NewManager(runner *remote.Runner, node config.NodeConfig) *Manager {
	return &Manager{
		runner: runner,
		node:   node,
		vmName: node.GetVMName(),
	}
}

// Status returns the current VM status on the remote host.
func (m *Manager) Status(ctx context.Context) (VMStatus, error) {
	out, err := m.runner.Run(ctx, "limactl list --json")
	if err != nil {
		if strings.Contains(err.Error(), "command not found") {
			return VMStatusNotCreated, fmt.Errorf("lima is not installed on %s", m.runner.Host)
		}
		return VMStatusUnknown, err
	}

	// limactl list --json outputs one JSON object per line.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Name == m.vmName {
			return VMStatus(entry.Status), nil
		}
	}

	return VMStatusNotCreated, nil
}

// GenerateConfig returns the Lima YAML config for this node.
func (m *Manager) GenerateConfig(socketPath string) string {
	return fmt.Sprintf(`vmType: "vz"
images:
  - location: "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-arm64.img"
    arch: "aarch64"
  - location: "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img"
    arch: "x86_64"
cpus: %d
memory: "%s"
disk: "%s"
networks:
  - socket: "%s"
provision:
  - mode: system
    script: |
      #!/bin/bash
      apt-get update -qq && apt-get install -y -qq curl open-iscsi nfs-common
`, m.node.VM.CPUs, m.node.VM.Memory, m.node.VM.Disk, socketPath)
}

// Provision creates and starts the Lima VM on the remote host.
func (m *Manager) Provision(ctx context.Context) error {
	status, err := m.Status(ctx)
	if err != nil && status != VMStatusNotCreated {
		return err
	}

	switch status {
	case VMStatusRunning:
		slog.Debug("VM already running, skipping provision", "host", m.runner.Host, "vm", m.vmName)
		fmt.Printf("  [%s] VM already running, skipping provision\n", m.runner.Host)
		return nil
	case VMStatusStopped:
		slog.Info("VM stopped, starting", "host", m.runner.Host, "vm", m.vmName)
		fmt.Printf("  [%s] VM exists but stopped, starting...\n", m.runner.Host)
		_, err := m.runner.Run(ctx, fmt.Sprintf("limactl start %s", m.vmName))
		return err
	case VMStatusNotCreated:
		slog.Info("creating and starting VM", "host", m.runner.Host, "vm", m.vmName,
			"cpus", m.node.VM.CPUs, "memory", m.node.VM.Memory, "disk", m.node.VM.Disk)
		fmt.Printf("  [%s] Creating and starting VM...\n", m.runner.Host)

		// Detect socket_vmnet socket path on the remote host.
		socketPath, err := m.detectSocketPath(ctx)
		if err != nil {
			return fmt.Errorf("detecting socket_vmnet path: %w", err)
		}

		// Write the config file to the remote host via base64 to avoid
		// shell quoting issues with multiline content.
		limaConfig := m.GenerateConfig(socketPath)
		configPath := fmt.Sprintf("~/%s.yaml", m.vmName)
		encoded := util.Base64Encode(limaConfig)
		_, err = m.runner.Run(ctx, fmt.Sprintf("echo %s | base64 -d > %s", encoded, configPath))
		if err != nil {
			return fmt.Errorf("writing lima config: %w", err)
		}

		// Start the VM with a generous timeout for boot scripts.
		_, err = m.runner.Run(ctx, fmt.Sprintf("limactl start --name=%s %s --tty=false --timeout=15m0s", m.vmName, configPath))
		if err != nil {
			// limactl may timeout even though the VM is running fine.
			// Check if the VM actually started despite the error.
			status, checkErr := m.Status(ctx)
			if checkErr == nil && status == VMStatusRunning {
				slog.Warn("limactl start returned error but VM is running", "host", m.runner.Host)
			} else {
				return fmt.Errorf("starting lima VM: %w", err)
			}
		}

		return nil
	default:
		return fmt.Errorf("unexpected VM status: %s", status)
	}
}

// GetBridgedIP returns the bridged LAN IP address of the VM.
func (m *Manager) GetBridgedIP(ctx context.Context) (string, error) {
	// Try lima0 interface first (socket_vmnet typically creates this).
	out, err := m.runner.LimaShell(ctx, m.vmName, "ip -4 addr show lima0 2>/dev/null | awk '/inet / {split($2,a,\"/\"); print a[1]}'")
	if err == nil && out != "" {
		return strings.TrimSpace(out), nil
	}

	// Fallback: use ip route to find the source IP for external traffic.
	out, err = m.runner.LimaShell(ctx, m.vmName, "ip -4 route get 1.1.1.1 | awk '/src/ {for(i=1;i<=NF;i++) if($i==\"src\") print $(i+1)}'")
	if err != nil {
		return "", fmt.Errorf("could not determine bridged IP: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// Destroy stops and deletes the Lima VM.
func (m *Manager) Destroy(ctx context.Context) error {
	status, err := m.Status(ctx)
	if err != nil && status != VMStatusNotCreated {
		return err
	}

	if status == VMStatusNotCreated {
		slog.Debug("no VM found, skipping destroy", "host", m.runner.Host, "vm", m.vmName)
		fmt.Printf("  [%s] No VM found, skipping\n", m.runner.Host)
		return nil
	}

	slog.Info("destroying VM", "host", m.runner.Host, "vm", m.vmName)
	fmt.Printf("  [%s] Destroying VM...\n", m.runner.Host)
	_, err = m.runner.Run(ctx, fmt.Sprintf("limactl stop %s --force 2>/dev/null; limactl delete %s --force", m.vmName, m.vmName))
	return err
}
// detectSocketPath finds the actual socket_vmnet socket on the remote host.
// The path varies by Homebrew installation: /opt/homebrew/var/run/ on ARM64,
// /usr/local/var/run/ on Intel.
func (m *Manager) detectSocketPath(ctx context.Context) (string, error) {
	candidates := []string{
		"/opt/homebrew/var/run/socket_vmnet",
		"/usr/local/var/run/socket_vmnet",
		"/var/run/socket_vmnet",
	}

	for _, path := range candidates {
		_, err := m.runner.RunShell(ctx, fmt.Sprintf("sudo test -S %s", path))
		if err == nil {
			slog.Debug("found socket_vmnet", "host", m.runner.Host, "path", path)
			return path, nil
		}
	}

	// Fallback: try to find it via brew --prefix.
	out, err := m.runner.RunShell(ctx, "brew --prefix 2>/dev/null")
	if err == nil {
		prefix := strings.TrimSpace(out)
		path := prefix + "/var/run/socket_vmnet"
		slog.Debug("using brew prefix for socket_vmnet", "host", m.runner.Host, "path", path)
		return path, nil
	}

	return "", fmt.Errorf("[%s] could not find socket_vmnet socket — ensure socket_vmnet is installed and running", m.runner.Host)
}

