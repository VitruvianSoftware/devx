# USB Stick Assembly + Flash Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `devx cluster usb build` so one command assembles a finished, bootable, self-joining Ventoy USB (download ISOs → embed Ignition → install Ventoy → partition 16 GB boot + exFAT storage → copy payloads) inside a Lima VM, and optionally flashes it to a removable device from macOS.

**Architecture:** All Linux-only work (Ventoy install, `coreos-installer iso ignition embed`, partitioning, exFAT format) runs inside an ephemeral **Lima builder VM** that produces a `.img`. macOS only performs the raw `dd` flash, behind removable-device guards. Pure units (size parsing, `diskutil` parsing, target validation, the generated provision/assembly scripts) are TDD'd; the shell-out orchestration uses an **injectable command runner** so command construction is unit-testable, with real Lima/flash runs gated behind tool presence.

**Tech Stack:** Go 1.25, cobra, Lima (`limactl`), Ventoy (Linux installer), `coreos-installer`, `parted`, `exfatprogs`, macOS `diskutil`/`dd`.

---

## File Structure

All under `internal/multinode/usb/` (flat package, matching the existing layout) plus the existing command file:

- `size.go` / `size_test.go` — `parseSizeMB` size helper.
- `flash.go` / `flash_test.go` — `DiskInfo`, `ParseDiskutilInfo`, `ValidateFlashTarget`, and `Flash` (injectable runner).
- `builder.go` / `builder_test.go` — `RenderBuilderProvision`, `EnsureBuilderVM`, `BuildImage` (injectable runner).
- `assemble_script.go` / `assemble_script_test.go` — `AssemblyParams`, `ISOSpec`, `RenderAssemblyScript`, ISO URL constants.
- `cmd/cluster_mgmt_usb.go` — extend `build` with `--device/--boot-size/--total-size/--image-out/--isos/--yes/--builder-vm/--keep-builder`, wire dry-run + flash.
- `internal/multinode/usb/testdata/*.golden` — golden files for the generated scripts.

Shared types introduced (used across tasks — keep names exact):

```go
// in flash.go
type DiskInfo struct {
	Identifier string // "disk4"
	MediaName  string // "SanDisk USB"
	Internal   bool
	Removable  bool
	Protocol   string // "USB"
	SizeBytes  int64
}

// in builder.go
type runFunc func(ctx context.Context, name string, args ...string) (string, error)

// in assemble_script.go
type ISOSpec struct {
	Name          string // "fcos" | "ubuntu"
	URL           string
	Filename      string
	EmbedIgnition bool
}
type AssemblyParams struct {
	BootSizeMB  int
	TotalSizeMB int
	ISOs        []ISOSpec
	IgnitionVM  string // path inside the VM to the .ign to embed
	PayloadVM   string // path inside the VM to staged ventoy.json + seeds
	ImageVM     string // path inside the VM for the output .img
}
```

---

## Task 1: Size parsing helper

**Files:**
- Create: `internal/multinode/usb/size.go`
- Test: `internal/multinode/usb/size_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/multinode/usb/size_test.go (license header omitted for brevity — copy from a sibling file)
package usb

import "testing"

func TestParseSizeMB(t *testing.T) {
	ok := map[string]int{"16G": 16384, "16GiB": 16384, "16GB": 16384, "512M": 512, "1g": 1024}
	for in, want := range ok {
		got, err := parseSizeMB(in)
		if err != nil || got != want {
			t.Errorf("parseSizeMB(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "16", "16K", "abc", "-4G", "0G"} {
		if _, err := parseSizeMB(bad); err == nil {
			t.Errorf("parseSizeMB(%q) should error", bad)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/multinode/usb/ -run TestParseSizeMB`
Expected: FAIL — `undefined: parseSizeMB`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/multinode/usb/size.go (add MIT license header from a sibling file)
package usb

import (
	"fmt"
	"strconv"
	"strings"
)

// parseSizeMB parses a size like "16G", "16GiB", "512M" into mebibytes
// (G = 1024 MiB). It requires an explicit M or G unit.
func parseSizeMB(s string) (int, error) {
	u := strings.ToUpper(strings.TrimSpace(s))
	u = strings.TrimSuffix(u, "IB")
	u = strings.TrimSuffix(u, "B")
	var mult int
	switch {
	case strings.HasSuffix(u, "G"):
		mult, u = 1024, strings.TrimSuffix(u, "G")
	case strings.HasSuffix(u, "M"):
		mult, u = 1, strings.TrimSuffix(u, "M")
	default:
		return 0, fmt.Errorf("size %q must end in M or G", s)
	}
	n, err := strconv.Atoi(strings.TrimSpace(u))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return n * mult, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/multinode/usb/ -run TestParseSizeMB`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/multinode/usb/size.go internal/multinode/usb/size_test.go
git add internal/multinode/usb/size.go internal/multinode/usb/size_test.go
git commit -m "feat(cluster): add size parser for usb image sizing"
```

---

## Task 2: diskutil parsing + flash-target validation

**Files:**
- Create: `internal/multinode/usb/flash.go`
- Test: `internal/multinode/usb/flash_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/multinode/usb/flash_test.go (add license header)
package usb

import "testing"

const sandiskInfo = `   Device Identifier:         disk4
   Device / Media Name:       SanDisk USB
   Protocol:                  USB
   Internal:                  No
   Removable Media:           Removable
   Disk Size:                 123.0 GB (123010547712 Bytes) (exactly 240254976 512-Byte-Units)`

const internalInfo = `   Device Identifier:         disk0
   Device / Media Name:       APPLE SSD
   Protocol:                  Apple Fabric
   Internal:                  Yes
   Removable Media:           Fixed
   Disk Size:                 994.7 GB (994662584320 Bytes)`

func TestParseDiskutilInfo(t *testing.T) {
	d := ParseDiskutilInfo(sandiskInfo)
	if d.Identifier != "disk4" || d.MediaName != "SanDisk USB" || d.Internal || !d.Removable || d.Protocol != "USB" {
		t.Fatalf("bad parse: %+v", d)
	}
	if d.SizeBytes != 123010547712 {
		t.Errorf("SizeBytes = %d, want 123010547712", d.SizeBytes)
	}
}

func TestValidateFlashTarget(t *testing.T) {
	if err := ValidateFlashTarget(ParseDiskutilInfo(sandiskInfo)); err != nil {
		t.Errorf("removable USB should be allowed: %v", err)
	}
	if err := ValidateFlashTarget(ParseDiskutilInfo(internalInfo)); err == nil {
		t.Error("internal disk must be refused")
	}
	if err := ValidateFlashTarget(DiskInfo{Identifier: "disk0", Removable: true}); err == nil {
		t.Error("disk0 must be refused even if it claims removable")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/multinode/usb/ -run 'Diskutil|FlashTarget'`
Expected: FAIL — undefined `ParseDiskutilInfo`, `ValidateFlashTarget`, `DiskInfo`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/multinode/usb/flash.go (add license header)
package usb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type DiskInfo struct {
	Identifier string
	MediaName  string
	Internal   bool
	Removable  bool
	Protocol   string
	SizeBytes  int64
}

var diskSizeBytesRE = regexp.MustCompile(`\((\d+) Bytes\)`)

// ParseDiskutilInfo parses the text output of `diskutil info <device>`.
func ParseDiskutilInfo(out string) DiskInfo {
	kv := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		k, v := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
		if k != "" {
			kv[k] = v
		}
	}
	d := DiskInfo{
		Identifier: kv["Device Identifier"],
		MediaName:  kv["Device / Media Name"],
		Internal:   strings.EqualFold(kv["Internal"], "Yes"),
		Protocol:   kv["Protocol"],
	}
	rm := strings.ToLower(kv["Removable Media"])
	d.Removable = strings.Contains(rm, "removable") || strings.EqualFold(kv["Ejectable"], "Yes")
	if m := diskSizeBytesRE.FindStringSubmatch(kv["Disk Size"]); m != nil {
		d.SizeBytes, _ = strconv.ParseInt(m[1], 10, 64)
	}
	return d
}

// ValidateFlashTarget refuses any target that is not clearly a removable,
// non-system, non-internal disk.
func ValidateFlashTarget(d DiskInfo) error {
	if d.Identifier == "disk0" {
		return fmt.Errorf("refusing to flash %s: that is the system disk", d.Identifier)
	}
	if d.Internal {
		return fmt.Errorf("refusing to flash %s (%s): internal disk", d.Identifier, d.MediaName)
	}
	if !d.Removable {
		return fmt.Errorf("refusing to flash %s (%s): not removable media", d.Identifier, d.MediaName)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/multinode/usb/ -run 'Diskutil|FlashTarget'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/multinode/usb/flash.go internal/multinode/usb/flash_test.go
git add internal/multinode/usb/flash.go internal/multinode/usb/flash_test.go
git commit -m "feat(cluster): parse diskutil info and validate flash targets"
```

---

## Task 3: Builder VM provision script generator

**Files:**
- Create: `internal/multinode/usb/builder.go`
- Test: `internal/multinode/usb/builder_test.go`
- Create (via -update): `internal/multinode/usb/testdata/builder-provision.sh.golden`

- [ ] **Step 1: Write the failing test**

```go
// internal/multinode/usb/builder_test.go (add license header)
package usb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderBuilderProvision_Golden(t *testing.T) {
	got := RenderBuilderProvision()
	golden := filepath.Join("testdata", "builder-provision.sh.golden")
	if *update {
		_ = os.WriteFile(golden, []byte(got), 0o644)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run -update): %v", err)
	}
	if got != string(want) {
		t.Errorf("provision script mismatch\n--- got ---\n%s", got)
	}
}

func TestRenderBuilderProvision_Tools(t *testing.T) {
	s := RenderBuilderProvision()
	for _, tool := range []string{"ventoy", "coreos-installer", "exfatprogs", "parted", "wget"} {
		if !strings.Contains(s, tool) {
			t.Errorf("provision script must install %q", tool)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/multinode/usb/ -run BuilderProvision`
Expected: FAIL — `undefined: RenderBuilderProvision`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/multinode/usb/builder.go (add license header)
package usb

// VentoyVersion / CoreosInstallerVersion pin the tools fetched into the builder.
const (
	VentoyVersion          = "1.0.99"
	CoreosInstallerVersion = "0.21.0"
)

// RenderBuilderProvision returns the Lima provision script that installs the
// Linux tooling needed to assemble the Ventoy image. Deterministic (golden).
func RenderBuilderProvision() string {
	return `#!/bin/bash
# devx usb builder provisioning — GENERATED.
set -euxo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq wget parted exfatprogs dosfstools util-linux ca-certificates

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
```

- [ ] **Step 4: Generate golden + run tests**

Run: `go test ./internal/multinode/usb/ -run BuilderProvision -update && go test ./internal/multinode/usb/ -run BuilderProvision`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/multinode/usb/builder.go internal/multinode/usb/builder_test.go
git add internal/multinode/usb/builder.go internal/multinode/usb/builder_test.go internal/multinode/usb/testdata/builder-provision.sh.golden
git commit -m "feat(cluster): generate usb builder VM provision script"
```

---

## Task 4: Assembly script generator

**Files:**
- Create: `internal/multinode/usb/assemble_script.go`
- Test: `internal/multinode/usb/assemble_script_test.go`
- Create (via -update): `internal/multinode/usb/testdata/assemble.sh.golden`

- [ ] **Step 1: Write the failing test**

```go
// internal/multinode/usb/assemble_script_test.go (add license header)
package usb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleAssembly() AssemblyParams {
	return AssemblyParams{
		BootSizeMB:  16384,
		TotalSizeMB: 117000,
		ISOs: []ISOSpec{
			{Name: "fcos", URL: "https://example/fcos.iso", Filename: "fcos.iso", EmbedIgnition: true},
			{Name: "ubuntu", URL: "https://example/ubuntu.iso", Filename: "ubuntu.iso"},
		},
		IgnitionVM: "/tmp/devx/payload/fcos.ign",
		PayloadVM:  "/tmp/devx/payload",
		ImageVM:    "/tmp/devx/devx-usb.img",
	}
}

func TestRenderAssemblyScript_Golden(t *testing.T) {
	got, err := RenderAssemblyScript(sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "assemble.sh.golden")
	if *update {
		_ = os.WriteFile(golden, []byte(got), 0o644)
		return
	}
	want, _ := os.ReadFile(golden)
	if got != string(want) {
		t.Errorf("assembly script mismatch\n--- got ---\n%s", got)
	}
}

func TestRenderAssemblyScript_Invariants(t *testing.T) {
	got, err := RenderAssemblyScript(sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"wget -q -O",                                   // downloads
		"coreos-installer iso ignition embed",          // embed
		"Ventoy2Disk.sh -I -r 100616",                  // reserve = total-boot = 117000-16384
		"mkfs.exfat",                                   // storage fs
		"truncate -s 117000M",                          // image size
	} {
		if !strings.Contains(got, want) {
			t.Errorf("assembly script must contain %q", want)
		}
	}
	// must NOT install to a host disk
	if strings.Contains(got, "coreos-installer install") || strings.Contains(got, "/dev/sda") {
		t.Error("assembly must not write to any physical/host disk")
	}
}

func TestRenderAssemblyScript_RequiresFCOS(t *testing.T) {
	p := sampleAssembly()
	p.ISOs = []ISOSpec{{Name: "ubuntu", URL: "u", Filename: "u.iso"}}
	if _, err := RenderAssemblyScript(p); err == nil {
		t.Error("an FCOS ISO with EmbedIgnition is required")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/multinode/usb/ -run AssemblyScript`
Expected: FAIL — undefined `AssemblyParams`, `ISOSpec`, `RenderAssemblyScript`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/multinode/usb/assemble_script.go (add license header)
package usb

import (
	"fmt"
	"strings"
)

// Default ephemeral ISO sources (x86_64; the feature targets x86 laptops).
const (
	UbuntuISOURL = "https://releases.ubuntu.com/24.04/ubuntu-24.04.1-live-server-amd64.iso"
	// FCOS uses stream metadata at runtime; FCOSStreamURL is resolved by the
	// assembly script (see resolveFCOSCmd) rather than hardcoded.
	FCOSStreamURL = "https://builds.coreos.fedoraproject.org/streams/stable.json"
)

type ISOSpec struct {
	Name          string
	URL           string
	Filename      string
	EmbedIgnition bool
}

type AssemblyParams struct {
	BootSizeMB  int
	TotalSizeMB int
	ISOs        []ISOSpec
	IgnitionVM  string
	PayloadVM   string
	ImageVM     string
}

// RenderAssemblyScript returns the deterministic bash run inside the Lima VM to
// produce the full Ventoy disk image. Pure text (golden-tested); its execution
// against real Ventoy is validated by the gated integration test in Task 8.
func RenderAssemblyScript(p AssemblyParams) (string, error) {
	var fcos *ISOSpec
	for i := range p.ISOs {
		if p.ISOs[i].EmbedIgnition {
			fcos = &p.ISOs[i]
		}
	}
	if fcos == nil {
		return "", fmt.Errorf("assembly requires one ISO with EmbedIgnition=true (FCOS)")
	}
	reserve := p.TotalSizeMB - p.BootSizeMB
	if reserve <= 0 {
		return "", fmt.Errorf("total size (%dM) must exceed boot size (%dM)", p.TotalSizeMB, p.BootSizeMB)
	}

	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	w("#!/usr/bin/env bash\n# devx usb assembly — GENERATED.\nset -euxo pipefail\n\n")
	w("CACHE=/var/cache/devx-usb\nmkdir -p \"$CACHE\"\n\n")
	for _, iso := range p.ISOs {
		w("if [ ! -f \"$CACHE/%s\" ]; then wget -q -O \"$CACHE/%s.part\" %q && mv \"$CACHE/%s.part\" \"$CACHE/%s\"; fi\n",
			iso.Filename, iso.Filename, iso.URL, iso.Filename, iso.Filename)
	}
	w("\n# Embed Ignition into a copy of the FCOS ISO\ncp \"$CACHE/%s\" /tmp/fcos-devx.iso\n", fcos.Filename)
	w("coreos-installer iso ignition embed -f -i %q /tmp/fcos-devx.iso\n\n", p.IgnitionVM)
	w("# Sparse image + loop\nrm -f %q\ntruncate -s %dM %q\nLOOP=$(losetup --show -f -P %q)\n\n", p.ImageVM, p.TotalSizeMB, p.ImageVM, p.ImageVM)
	w("# Install Ventoy (force, reserve the storage tail)\nyes | bash /opt/ventoy/Ventoy2Disk.sh -I -r %d \"$LOOP\"\npartprobe \"$LOOP\" || true\nsleep 2\n\n", reserve)
	w("# Storage partition in the reserved tail → exFAT (3rd primary in free space)\n")
	w("parted -s \"$LOOP\" -- mkpart primary 3 -1s\npartprobe \"$LOOP\" || true\nsleep 2\nmkfs.exfat -n DEVXDATA \"${LOOP}p3\"\n\n")
	w("# Copy ISOs + payloads onto the Ventoy data partition\nmkdir -p /mnt/vtoy\nmount \"${LOOP}p1\" /mnt/vtoy\n")
	w("cp /tmp/fcos-devx.iso /mnt/vtoy/%s\n", fcos.Filename)
	for _, iso := range p.ISOs {
		if iso.EmbedIgnition {
			continue
		}
		w("cp \"$CACHE/%s\" /mnt/vtoy/%s\n", iso.Filename, iso.Filename)
	}
	w("mkdir -p /mnt/vtoy/ventoy\ncp %q/ventoy.json /mnt/vtoy/ventoy/ventoy.json\n", p.PayloadVM)
	w("cp -r %q/seed /mnt/vtoy/ 2>/dev/null || true\n", p.PayloadVM)
	w("sync\numount /mnt/vtoy\nlosetup -d \"$LOOP\"\necho \"devx: image ready at %s\"\n", p.ImageVM)
	return b.String(), nil
}
```

> Note for the implementer: the `parted ... mkpart primary 3 -1s` and `${LOOP}p3`
> partition mechanics are the part most likely to need iteration against real
> Ventoy in Task 8 (Ventoy's exact partition count/numbering can vary by version).
> Keep the *generated text* deterministic; adjust the template, regenerate the
> golden, and re-run Task 8 if the integration test shows a mismatch.

- [ ] **Step 4: Generate golden + run tests**

Run: `go test ./internal/multinode/usb/ -run AssemblyScript -update && go test ./internal/multinode/usb/ -run AssemblyScript`
Expected: PASS (golden created; invariants hold).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/multinode/usb/assemble_script.go internal/multinode/usb/assemble_script_test.go
git add internal/multinode/usb/assemble_script.go internal/multinode/usb/assemble_script_test.go internal/multinode/usb/testdata/assemble.sh.golden
git commit -m "feat(cluster): generate in-VM Ventoy image assembly script"
```

---

## Task 5: Flash execution (injectable runner)

**Files:**
- Modify: `internal/multinode/usb/flash.go`
- Test: `internal/multinode/usb/flash_test.go`

- [ ] **Step 1: Write the failing test (append to flash_test.go)**

```go
func TestFlashCommands(t *testing.T) {
	var calls [][]string
	fake := func(ctx context.Context, name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name}, args...))
		if name == "diskutil" && len(args) > 0 && args[0] == "info" {
			return sandiskInfo, nil
		}
		return "", nil
	}
	err := flashWith(context.Background(), fake, "/tmp/x.img", "/dev/disk4", func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, c := range calls {
		joined += strings.Join(c, " ") + "\n"
	}
	for _, want := range []string{
		"diskutil info /dev/disk4",
		"diskutil unmountDisk /dev/disk4",
		"dd if=/tmp/x.img of=/dev/rdisk4",
		"diskutil eject /dev/disk4",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing command %q in:\n%s", want, joined)
		}
	}
}

func TestFlashRefusesInternalDisk(t *testing.T) {
	fake := func(ctx context.Context, name string, args ...string) (string, error) {
		return internalInfo, nil
	}
	if err := flashWith(context.Background(), fake, "/tmp/x.img", "/dev/disk0", func() bool { return true }); err == nil {
		t.Error("must refuse to flash an internal disk")
	}
}

func TestFlashAbortsWithoutConfirm(t *testing.T) {
	fake := func(ctx context.Context, name string, args ...string) (string, error) { return sandiskInfo, nil }
	if err := flashWith(context.Background(), fake, "/tmp/x.img", "/dev/disk4", func() bool { return false }); err == nil {
		t.Error("must abort when confirmation is declined")
	}
}
```

Add imports `context` and `strings` to `flash_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/multinode/usb/ -run Flash`
Expected: FAIL — `undefined: flashWith`.

- [ ] **Step 3: Write minimal implementation (append to flash.go)**

```go
import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Flash validates the target device and writes imagePath to it. confirm is
// called (unless skipped by the caller) to gate the destructive write.
func Flash(ctx context.Context, imagePath, device string, confirm func() bool) error {
	return flashWith(ctx, execRun, imagePath, device, confirm)
}

func execRun(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

func flashWith(ctx context.Context, run runFunc, imagePath, device string, confirm func() bool) error {
	infoOut, err := run(ctx, "diskutil", "info", device)
	if err != nil {
		return fmt.Errorf("diskutil info %s: %w", device, err)
	}
	info := ParseDiskutilInfo(infoOut)
	if err := ValidateFlashTarget(info); err != nil {
		return err
	}
	if confirm != nil && !confirm() {
		return fmt.Errorf("flash aborted by user")
	}
	if _, err := run(ctx, "diskutil", "unmountDisk", device); err != nil {
		return fmt.Errorf("unmount %s: %w", device, err)
	}
	raw := strings.Replace(device, "/dev/disk", "/dev/rdisk", 1) // raw device = much faster
	if _, err := run(ctx, "dd", "if="+imagePath, "of="+raw, "bs=4m"); err != nil {
		return fmt.Errorf("dd to %s: %w", raw, err)
	}
	_, _ = run(ctx, "diskutil", "eject", device)
	return nil
}
```

> `runFunc` is defined in builder.go (Task 6). If implementing Task 5 before
> Task 6, add `type runFunc = func(ctx context.Context, name string, args ...string) (string, error)`
> to flash.go and remove it from builder.go (keep exactly one definition).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/multinode/usb/ -run Flash`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/multinode/usb/flash.go internal/multinode/usb/flash_test.go
git add internal/multinode/usb/flash.go internal/multinode/usb/flash_test.go
git commit -m "feat(cluster): flash image to a validated removable device"
```

---

## Task 6: Lima builder orchestration (injectable runner)

**Files:**
- Modify: `internal/multinode/usb/builder.go`
- Test: `internal/multinode/usb/builder_test.go`

- [ ] **Step 1: Write the failing test (append to builder_test.go)**

```go
import "context"

func TestBuildImageCommands(t *testing.T) {
	var calls [][]string
	fake := func(ctx context.Context, name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name}, args...))
		return "", nil
	}
	b := &Builder{VMName: "devx-usb-builder", run: fake}
	_, err := b.BuildImage(context.Background(), AssemblyParams{
		BootSizeMB: 16384, TotalSizeMB: 117000,
		ISOs:       []ISOSpec{{Name: "fcos", URL: "u", Filename: "f.iso", EmbedIgnition: true}},
		IgnitionVM: "/tmp/devx/payload/f.ign", PayloadVM: "/tmp/devx/payload", ImageVM: "/tmp/devx/devx-usb.img",
	}, "/local/staging")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, c := range calls {
		joined += strings.Join(c, " ") + "\n"
	}
	for _, want := range []string{
		"limactl shell devx-usb-builder",      // runs assembly in the VM
		"limactl copy",                        // copies staging in and image out
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/multinode/usb/ -run BuildImageCommands`
Expected: FAIL — `undefined: Builder`.

- [ ] **Step 3: Write minimal implementation (append to builder.go)**

```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type runFunc func(ctx context.Context, name string, args ...string) (string, error)

type Builder struct {
	VMName string
	run    runFunc
}

func NewBuilder(vmName string) *Builder {
	return &Builder{VMName: vmName, run: func(ctx context.Context, name string, args ...string) (string, error) {
		return execRun(ctx, name, args...) // execRun defined in flash.go
	}}
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
```

> `EnsureBuilderVM` (creating/starting the Lima VM with `RenderBuilderProvision`)
> shells out to `limactl start` with a generated lima.yaml; it is exercised only
> by the gated integration test (Task 8) and the real run, not unit-tested. Add it
> as a thin method `func (b *Builder) EnsureBuilderVM(ctx context.Context) error`
> that writes a temp lima.yaml embedding `RenderBuilderProvision()` and runs
> `limactl start --tty=false --name <vm> <yaml>` (idempotent: skip if
> `limactl list <vm>` shows Running).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/multinode/usb/ -run BuildImageCommands`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/multinode/usb/builder.go internal/multinode/usb/builder_test.go
git add internal/multinode/usb/builder.go internal/multinode/usb/builder_test.go
git commit -m "feat(cluster): orchestrate Ventoy image build in a Lima VM"
```

---

## Task 7: Wire `--device` and friends into `cluster usb build`

**Files:**
- Modify: `cmd/cluster_mgmt_usb.go`

- [ ] **Step 1: Add flags + plumbing**

In `newClusterUSBBuildCmd`, add flags after the existing ones:

```go
var (
	device      string
	bootSize    string
	totalSize   string
	imageOut    string
	isoList     []string
	assumeYes   bool
	builderVM   string
	keepBuilder bool
)
cmd.Flags().StringVar(&device, "device", "", "flash the assembled image to this removable device (e.g. /dev/disk4)")
cmd.Flags().StringVar(&bootSize, "boot-size", "16G", "Ventoy boot partition size; remainder becomes exFAT storage")
cmd.Flags().StringVar(&totalSize, "total-size", "", "total image size (defaults to the --device size, else 32G)")
cmd.Flags().StringVar(&imageOut, "image-out", "devx-usb.img", "where to write the assembled image when --device is not set")
cmd.Flags().StringSliceVar(&isoList, "isos", []string{"fcos", "ubuntu"}, "ISOs to include (ephemeral)")
cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the destructive-write confirmation")
cmd.Flags().StringVar(&builderVM, "builder-vm", "devx-usb-builder", "name of the Lima builder VM")
cmd.Flags().BoolVar(&keepBuilder, "keep-builder", false, "do not delete the builder VM afterward")
```

- [ ] **Step 2: In the RunE, after rendering payloads, branch to assemble/flash**

```go
// After the existing usb.Build(...) staging step produced the payload dir `outDir`:
bootMB, err := parseSizeMB(bootSize)
if err != nil {
	return err
}
totalMB := 32 * 1024
if totalSize != "" {
	if totalMB, err = parseSizeMB(totalSize); err != nil {
		return err
	}
}
if device != "" && totalSize == "" {
	infoOut, _ := exec.CommandContext(ctx, "diskutil", "info", device).CombinedOutput()
	if d := usb.ParseDiskutilInfo(string(infoOut)); d.SizeBytes > 0 {
		totalMB = int(d.SizeBytes / (1024 * 1024))
	}
}
if DryRun {
	fmt.Printf("📋 Dry-run: would assemble %dM image (boot %dM) from ISOs %v\n", totalMB, bootMB, isoList)
	if device != "" {
		fmt.Printf("           then flash to %s (after confirmation)\n", device)
	}
	return nil
}

params := usb.AssemblyParams{
	BootSizeMB: bootMB, TotalSizeMB: totalMB,
	ISOs:       usb.DefaultISOSpecs(isoList), // fcos+ubuntu defaults
	IgnitionVM: "/tmp/devx/payload/fcos/ephemeral/devx-join.ign",
	PayloadVM:  "/tmp/devx/payload",
	ImageVM:    "/tmp/devx/devx-usb.img",
}
b := usb.NewBuilder(builderVM)
if err := b.EnsureBuilderVM(ctx); err != nil {
	return err
}
img, err := b.BuildImage(ctx, params, outDir)
if err != nil {
	return err
}
if device == "" {
	return os.Rename(img, imageOut)
}
confirm := func() bool {
	if assumeYes {
		return true
	}
	fmt.Printf("⚠️  About to ERASE %s and flash the Ventoy image. Type 'yes' to continue: ", device)
	var s string
	_, _ = fmt.Scanln(&s)
	return s == "yes"
}
return usb.Flash(ctx, img, device, confirm)
```

Add `usb.DefaultISOSpecs([]string) []ISOSpec` to `assemble_script.go` returning the FCOS (EmbedIgnition) + Ubuntu specs for the requested names. Note: the FCOS `.ign` must be compiled from the renderer's `.bu` during staging — extend the FCOS staging in `usb.Build` to also emit `devx-join.ign` via `internal/ignition` when `butane` is available, or have the assembly script compile it (butane is in the builder VM — simpler: add butane to the provision script and compile in-VM). Choose **compile in-VM**: add `butane` to Task 3's provision and have the assembly script run `butane --strict -o <ign> <bu>` before the embed step.

- [ ] **Step 3: Build + smoke (dry-run only — no VM/flash)**

Run: `go build ./... && go run . -c <cluster.yaml> cluster usb build --dry-run --device /dev/disk4`
Expected: prints the assemble+flash plan, touches nothing.

- [ ] **Step 4: Commit**

```bash
gofmt -w cmd/cluster_mgmt_usb.go internal/multinode/usb/assemble_script.go
git add cmd/cluster_mgmt_usb.go internal/multinode/usb/assemble_script.go
git commit -m "feat(cluster): wire usb build --device assemble+flash path"
```

---

## Task 8: Gated end-to-end integration (real Lima build + image inspection)

**Files:**
- Create: `internal/multinode/usb/assemble_integration_test.go`

- [ ] **Step 1: Write the gated integration test**

```go
//go:build integration

// Run with: go test -tags integration ./internal/multinode/usb/ -run Integration -timeout 30m
package usb

import (
	"context"
	"os/exec"
	"testing"
)

func TestAssembleIntegration(t *testing.T) {
	if _, err := exec.LookPath("limactl"); err != nil {
		t.Skip("limactl not present")
	}
	// Build a small image (no flash), then loopback-inspect it INSIDE the VM.
	b := NewBuilder("devx-usb-itest")
	if err := b.EnsureBuilderVM(context.Background()); err != nil {
		t.Fatal(err)
	}
	// ... assemble a 6G image with both ISOs, then assert via limactl shell:
	//   - `losetup`/`parted -l` shows a Ventoy data partition + an exFAT DEVXDATA partition
	//   - both ISO filenames exist on the mounted Ventoy partition
	//   - `coreos-installer iso ignition show` on the embedded FCOS ISO returns the config
	// Fail the test if any assertion fails; clean up the VM at the end.
}
```

- [ ] **Step 2: Run it (manual / CI opt-in)**

Run: `go test -tags integration ./internal/multinode/usb/ -run Integration -timeout 30m`
Expected: PASS, or a clear failure pointing at the assembly step to iterate (adjust the Task 4 template + regenerate golden as needed).

- [ ] **Step 3: Commit**

```bash
git add internal/multinode/usb/assemble_integration_test.go
git commit -m "test(cluster): gated end-to-end usb assembly integration test"
```

---

## Task 9: Final verification + docs

- [ ] **Step 1: Full preflight**

```bash
go build ./...
go test ./internal/multinode/usb/ ./cmd/
golangci-lint run ./internal/multinode/usb/... ./cmd/...
```
Expected: build ok, tests pass, 0 lint issues.

- [ ] **Step 2: Update the MANIFEST/help text**

Update `usb.manifest(...)` (in `build.go`) and the `usb build` long help to mention `--device`, the 16 GB boot + exFAT storage layout, and that the assembled stick is ephemeral-only.

- [ ] **Step 3: Commit + open PR for review**

```bash
git add -A && git commit -m "docs(cluster): note --device assemble+flash in usb help/manifest"
# open PR (CI-gated); leave for review per merge-review policy
go run . agent review --skip-preflight -m "feat(cluster): one-command USB assembly + flash (build --device)"
```

- [ ] **Step 4: Real-hardware validation (operator)**

Run on the actual SanDisk: `devx cluster usb build --device /dev/disk4`, then boot an x86 laptop off it and confirm the node joins (`kubectl get nodes`). This is the field test the automated suite cannot perform on arm64.

---

## Self-Review

- **Spec coverage:** §4 command surface → Task 7. §5 components → Tasks 2–6. §3 flash safety → Tasks 2,5. §3 host-disk safety (ephemeral-only) → enforced by Task 7 forcing ephemeral + Task 4 invariant rejecting host-disk writes. §6 data flow → Tasks 4,6,7. §7 testing → Tasks 1–8. §9 FCOS URL resolution → noted in Task 4 (stream JSON) + Task 7 (compile-in-VM). All covered.
- **Placeholder scan:** Two intentional prose notes (Task 6 `EnsureBuilderVM` sketch, Task 8 inspection asserts) describe shell-out/integration glue that is validated by the gated integration test, not unit-tested — acceptable, with concrete commands given. No `TBD`/`TODO` in code steps.
- **Type consistency:** `runFunc`, `Builder{VMName,run}`, `AssemblyParams`, `ISOSpec`, `DiskInfo`, `ParseDiskutilInfo`, `ValidateFlashTarget`, `Flash/flashWith`, `RenderAssemblyScript`, `RenderBuilderProvision`, `parseSizeMB`, `DefaultISOSpecs` — names consistent across tasks. One flagged hazard: `runFunc` must be defined exactly once (Task 5 note) — keep it in builder.go.
