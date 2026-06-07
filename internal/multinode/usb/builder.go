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
	"strings"
)

// VentoyVersion pins the Ventoy Linux installer fetched into the builder VM.
const VentoyVersion = "1.0.99"

// runFunc is an injectable command runner so orchestration command-construction
// can be unit-tested without executing anything. The production runner is execRun.
type runFunc func(ctx context.Context, name string, args ...string) (string, error)

// RenderBuilderProvision returns the Lima provision script that installs the
// Linux tooling needed to assemble the Ventoy image. Deterministic (golden).
// coreos-installer is NOT installed here — it ships no Linux binary on GitHub, so
// the assembly script runs its official container via podman.
func RenderBuilderProvision() string {
	return `#!/bin/bash
# devx usb builder provisioning — GENERATED.
set -euxo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq wget jq parted exfatprogs dosfstools util-linux ca-certificates butane podman

# Ventoy (Linux installer)
if [ ! -d /opt/ventoy ]; then
  wget -qO /tmp/ventoy.tar.gz https://github.com/ventoy/Ventoy/releases/download/v` + VentoyVersion + `/ventoy-` + VentoyVersion + `-linux.tar.gz
  mkdir -p /opt/ventoy
  tar -xzf /tmp/ventoy.tar.gz -C /opt/ventoy --strip-components=1
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
// RenderBuilderProvision) if needed. It resumes a stopped-but-existing VM and
// only generates a fresh config when the VM is absent. Idempotent.
func (b *Builder) EnsureBuilderVM(ctx context.Context) error {
	out, _ := b.run(ctx, "limactl", "list", b.VMName, "--format", "{{.Status}}")
	switch strings.TrimSpace(out) {
	case "Running":
		return nil
	case "":
		// Absent — create below.
	default:
		// Exists but stopped — resume without re-passing a config.
		if _, err := b.run(ctx, "limactl", "start", b.VMName); err != nil {
			return fmt.Errorf("resuming builder VM %s: %w", b.VMName, err)
		}
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

// BuildImage tars the staging payloads, copies them + a generated assembly script
// into the builder VM, runs it, and copies the resulting image back to a unique
// host temp file (the caller owns its cleanup). Tarring avoids limactl-copy
// directory-nesting ambiguity and the need for a recursive copy flag.
func (b *Builder) BuildImage(ctx context.Context, p AssemblyParams, stagingDir string) (string, error) {
	script, err := RenderAssemblyScript(p)
	if err != nil {
		return "", err
	}

	scriptF, err := os.CreateTemp("", "devx-assemble-*.sh")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(scriptF.Name()) }()
	if _, err := scriptF.WriteString(script); err != nil {
		return "", err
	}
	_ = scriptF.Close()

	tarF, err := os.CreateTemp("", "devx-staging-*.tgz")
	if err != nil {
		return "", err
	}
	tarPath := tarF.Name()
	_ = tarF.Close()
	defer func() { _ = os.Remove(tarPath) }()
	if _, err := b.run(ctx, "tar", "czf", tarPath, "-C", stagingDir, "."); err != nil {
		return "", fmt.Errorf("tarring staging dir: %w", err)
	}

	// Stage inside the VM as the lima user (writable), extract, then run as root.
	if _, err := b.run(ctx, "limactl", "shell", b.VMName, "mkdir", "-p", "/tmp/devx/payload"); err != nil {
		return "", fmt.Errorf("mkdir in VM: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "copy", tarPath, b.VMName+":/tmp/devx/payload.tgz"); err != nil {
		return "", fmt.Errorf("copy staging tarball: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "shell", b.VMName, "tar", "xzf", "/tmp/devx/payload.tgz", "-C", "/tmp/devx/payload"); err != nil {
		return "", fmt.Errorf("extract staging in VM: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "copy", scriptF.Name(), b.VMName+":/tmp/devx/assemble.sh"); err != nil {
		return "", fmt.Errorf("copy assembly script: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "shell", b.VMName, "sudo", "bash", "/tmp/devx/assemble.sh"); err != nil {
		return "", fmt.Errorf("run assembly: %w", err)
	}

	imgF, err := os.CreateTemp("", "devx-usb-*.img")
	if err != nil {
		return "", err
	}
	hostImg := imgF.Name()
	_ = imgF.Close()
	if _, err := b.run(ctx, "limactl", "copy", b.VMName+":"+p.ImageVM, hostImg); err != nil {
		_ = os.Remove(hostImg)
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
