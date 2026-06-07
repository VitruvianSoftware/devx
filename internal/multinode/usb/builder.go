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

package usb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VentoyVersion / CoreosInstallerVersion pin the tools fetched into the builder VM.
const (
	VentoyVersion          = "1.0.99"
	CoreosInstallerVersion = "0.21.0"
)

// runFunc is an injectable command runner so orchestration command-construction
// can be unit-tested without executing anything. The production runner is execRun.
type runFunc func(ctx context.Context, name string, args ...string) (string, error)

// RenderBuilderProvision returns the Lima provision script that installs the
// Linux tooling needed to assemble the Ventoy image. Deterministic (golden).
func RenderBuilderProvision() string {
	return `#!/bin/bash
# devx usb builder provisioning — GENERATED.
set -euxo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq wget jq parted exfatprogs dosfstools util-linux ca-certificates butane

# Ventoy (Linux installer)
if [ ! -d /opt/ventoy ]; then
  wget -qO /tmp/ventoy.tar.gz https://github.com/ventoy/Ventoy/releases/download/v` + VentoyVersion + `/ventoy-` + VentoyVersion + `-linux.tar.gz
  mkdir -p /opt/ventoy
  tar -xzf /tmp/ventoy.tar.gz -C /opt/ventoy --strip-components=1
fi

# coreos-installer (static binary)
if ! command -v coreos-installer >/dev/null 2>&1; then
  wget -qO /usr/local/bin/coreos-installer https://github.com/coreos/coreos-installer/releases/download/v` + CoreosInstallerVersion + `/coreos-installer-$(uname -m)-unknown-linux-gnu
  chmod 0755 /usr/local/bin/coreos-installer
fi
echo "devx: builder provisioned"
`
}

// Builder orchestrates the Ventoy image assembly inside a Lima VM.
type Builder struct {
	VMName string
	run    runFunc
}

// NewBuilder returns a Builder that shells out via the system command runner.
func NewBuilder(vmName string) *Builder {
	return &Builder{VMName: vmName, run: execRun}
}

// EnsureBuilderVM creates and starts the builder VM (provisioned via
// RenderBuilderProvision) if it is not already running. Idempotent.
func (b *Builder) EnsureBuilderVM(ctx context.Context) error {
	if out, err := b.run(ctx, "limactl", "list", b.VMName, "--format", "{{.Status}}"); err == nil && strings.Contains(out, "Running") {
		return nil
	}
	yaml := fmt.Sprintf(`images:
  - location: "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img"
    arch: "x86_64"
  - location: "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-arm64.img"
    arch: "aarch64"
cpus: 4
memory: "6GiB"
disk: "60GiB"
mounts: []
provision:
  - mode: system
    script: |
%s
`, indent(RenderBuilderProvision(), "      "))

	f, err := os.CreateTemp("", "devx-usb-builder-*.yaml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString(yaml); err != nil {
		return err
	}
	_ = f.Close()
	if _, err := b.run(ctx, "limactl", "start", "--tty=false", "--name="+b.VMName, f.Name()); err != nil {
		return fmt.Errorf("starting builder VM %s: %w", b.VMName, err)
	}
	return nil
}

// BuildImage copies the staging payloads + a generated assembly script into the
// builder VM, runs it, and copies the resulting image back to a host temp file.
func (b *Builder) BuildImage(ctx context.Context, p AssemblyParams, stagingDir string) (string, error) {
	script, err := RenderAssemblyScript(p)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp("", "devx-assemble-*.sh")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(script); err != nil {
		return "", err
	}
	_ = tmp.Close()

	if _, err := b.run(ctx, "limactl", "copy", stagingDir, b.VMName+":/tmp/devx/payload"); err != nil {
		return "", fmt.Errorf("copy staging into VM: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "copy", tmp.Name(), b.VMName+":/tmp/devx/assemble.sh"); err != nil {
		return "", fmt.Errorf("copy assembly script: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "shell", b.VMName, "sudo", "bash", "/tmp/devx/assemble.sh"); err != nil {
		return "", fmt.Errorf("run assembly: %w", err)
	}
	hostImg := filepath.Join(os.TempDir(), "devx-usb.img")
	if _, err := b.run(ctx, "limactl", "copy", b.VMName+":"+p.ImageVM, hostImg); err != nil {
		return "", fmt.Errorf("copy image out: %w", err)
	}
	return hostImg, nil
}

// indent prefixes every line of s with the given pad (for embedding a script
// into a YAML block scalar).
func indent(s, pad string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}
