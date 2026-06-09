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

// ButaneVersion is fetched into the builder VM as a static binary (not an apt
// package). butane compiles the join Butane into the Ignition that
// coreos-installer embeds.
const ButaneVersion = "0.23.0"

// runFunc is an injectable command runner so orchestration command-construction
// can be unit-tested without executing anything. The production runner is execRun.
type runFunc func(ctx context.Context, name string, args ...string) (string, error)

// RenderBuilderProvision returns the Lima provision script that installs the
// Linux tooling needed to assemble the GPT Fedora CoreOS image. Deterministic
// (golden). coreos-installer is NOT installed here — it ships no Linux binary on
// GitHub, so the assembly script runs its official container via podman.
func RenderBuilderProvision() string {
	return `#!/bin/bash
# devx usb builder provisioning — GENERATED. Idempotent; run via limactl shell.
set -euxo pipefail
export DEBIAN_FRONTEND=noninteractive
# apt-get update can race first-boot networking; retry briefly.
for i in 1 2 3 4 5; do apt-get update -qq && break; sleep 5; done
apt-get install -y -qq wget util-linux ca-certificates podman

# Butane (Ignition compiler) — not in apt, but ships a static Linux binary.
if ! command -v butane >/dev/null 2>&1; then
  wget -qO /usr/local/bin/butane https://github.com/coreos/butane/releases/download/v` + ButaneVersion + `/butane-$(uname -m)-unknown-linux-gnu
  chmod 0755 /usr/local/bin/butane
fi
echo "devx: builder provisioned"
`
}

// Builder orchestrates the GPT Fedora CoreOS image assembly inside a Lima VM.
type Builder struct {
	VMName string
	run    runFunc
}

// NewBuilder returns a Builder that shells out via the system command runner.
func NewBuilder(vmName string) *Builder {
	return &Builder{VMName: vmName, run: execRun}
}

// builderVMYAML is a bare Ubuntu 24.04 builder VM. Provisioning is NOT embedded
// here: Lima's YAML-block first-boot provisioning is fragile (a failure can be
// swallowed and `limactl start` still report success), so EnsureBuilderVM runs
// the provision script explicitly via limactl shell instead.
const builderVMYAML = `images:
  - location: "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img"
    arch: "x86_64"
  - location: "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-arm64.img"
    arch: "aarch64"
cpus: 4
memory: "6GiB"
disk: "60GiB"
mounts: []
`

// EnsureBuilderVM ensures the builder VM exists and is provisioned. It creates a
// bare VM when absent, resumes a stopped one, then runs the (idempotent)
// provision script explicitly so failures surface as errors. Idempotent.
func (b *Builder) EnsureBuilderVM(ctx context.Context) error {
	out, _ := b.run(ctx, "limactl", "list", b.VMName, "--format", "{{.Status}}")
	switch strings.TrimSpace(out) {
	case "Running":
		// running — (re)provision below.
	case "":
		f, err := os.CreateTemp("", "devx-usb-builder-*.yaml")
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(f.Name()) }()
		if _, err := f.WriteString(builderVMYAML); err != nil {
			return err
		}
		_ = f.Close()
		if _, err := b.run(ctx, "limactl", "start", "--tty=false", "--name="+b.VMName, f.Name()); err != nil {
			return fmt.Errorf("starting builder VM %s: %w", b.VMName, err)
		}
	default:
		if _, err := b.run(ctx, "limactl", "start", b.VMName); err != nil {
			return fmt.Errorf("resuming builder VM %s: %w", b.VMName, err)
		}
	}

	// Provision explicitly (idempotent) so failures are visible and retriable.
	pf, err := os.CreateTemp("", "devx-provision-*.sh")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(pf.Name()) }()
	if _, err := pf.WriteString(RenderBuilderProvision()); err != nil {
		return err
	}
	_ = pf.Close()
	if _, err := b.run(ctx, "limactl", "copy", pf.Name(), b.VMName+":/tmp/devx-provision.sh"); err != nil {
		return fmt.Errorf("copying provision script: %w", err)
	}
	if out, err := b.run(ctx, "limactl", "shell", b.VMName, "sudo", "bash", "/tmp/devx-provision.sh"); err != nil {
		return fmt.Errorf("provisioning builder VM %s: %w\n%s", b.VMName, err, out)
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
	if _, err := b.run(ctx, "limactl", "shell", b.VMName, "mkdir", "-p", "/var/tmp/devx/payload"); err != nil {
		return "", fmt.Errorf("mkdir in VM: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "copy", tarPath, b.VMName+":/var/tmp/devx/payload.tgz"); err != nil {
		return "", fmt.Errorf("copy staging tarball: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "shell", b.VMName, "tar", "xzf", "/var/tmp/devx/payload.tgz", "-C", "/var/tmp/devx/payload"); err != nil {
		return "", fmt.Errorf("extract staging in VM: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "copy", scriptF.Name(), b.VMName+":/var/tmp/devx/assemble.sh"); err != nil {
		return "", fmt.Errorf("copy assembly script: %w", err)
	}
	if _, err := b.run(ctx, "limactl", "shell", b.VMName, "sudo", "bash", "/var/tmp/devx/assemble.sh"); err != nil {
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
	// Reclaim the in-VM working dir (image + payload, multi-GB).
	_, _ = b.run(ctx, "limactl", "shell", b.VMName, "sudo", "rm", "-rf", "/var/tmp/devx")
	return hostImg, nil
}
