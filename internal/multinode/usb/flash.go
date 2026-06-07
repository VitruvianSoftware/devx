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
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DiskInfo is the subset of `diskutil info <device>` output relevant to deciding
// whether a target is a safe, removable flash destination.
type DiskInfo struct {
	Identifier string // "disk4"
	MediaName  string // "SanDisk USB"
	Internal   bool
	Removable  bool
	Protocol   string // "USB"
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
	// Gate strictly on "Removable Media: Removable" — do NOT infer removability
	// from Ejectable. External SSDs/HDDs (a Time Machine drive, a portable SSD)
	// are Ejectable but report "Fixed"; erasing one would be data loss.
	d.Removable = strings.EqualFold(strings.TrimSpace(kv["Removable Media"]), "Removable")
	if m := diskSizeBytesRE.FindStringSubmatch(kv["Disk Size"]); m != nil {
		d.SizeBytes, _ = strconv.ParseInt(m[1], 10, 64)
	}
	return d
}

// wholeDiskRE matches a whole-disk identifier ("disk4"), not a partition
// ("disk4s2") — flashing a whole-disk image into a partition would corrupt it.
var wholeDiskRE = regexp.MustCompile(`^disk[0-9]+$`)

// ValidateFlashTarget refuses any target that is not positively identified as a
// removable, non-system, whole USB/SD stick. Unknown/unparseable attributes are
// treated as unsafe (fail closed).
func ValidateFlashTarget(d DiskInfo) error {
	if !wholeDiskRE.MatchString(d.Identifier) {
		return fmt.Errorf("refusing to flash %q: not a whole disk (pass /dev/diskN, not a partition)", d.Identifier)
	}
	if d.Identifier == "disk0" {
		return fmt.Errorf("refusing to flash %s: that is the system disk", d.Identifier)
	}
	if d.Internal {
		return fmt.Errorf("refusing to flash %s (%s): internal disk", d.Identifier, d.MediaName)
	}
	if d.SizeBytes == 0 {
		return fmt.Errorf("refusing to flash %s: could not confirm it is a removable stick (no size from diskutil)", d.Identifier)
	}
	if !d.Removable {
		return fmt.Errorf("refusing to flash %s (%s): not removable media", d.Identifier, d.MediaName)
	}
	if !isRemovableProtocol(d.Protocol) {
		return fmt.Errorf("refusing to flash %s (%s): protocol %q is not USB/SD", d.Identifier, d.MediaName, d.Protocol)
	}
	return nil
}

func isRemovableProtocol(p string) bool {
	p = strings.ToUpper(p)
	return strings.Contains(p, "USB") || strings.Contains(p, "SECURE DIGITAL")
}

// Flash validates the target device and writes imagePath to it. confirm is
// called (unless the caller passes a nil/always-true confirm) to gate the
// destructive write.
func Flash(ctx context.Context, imagePath, device string, confirm func() bool) error {
	return flashWith(ctx, execRun, imagePath, device, confirm)
}

// execRun runs an external command and returns its combined output.
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
	// Echo the resolved target so the operator can cross-check before confirming.
	fmt.Printf("  Target: /dev/%s — %s, %.1f GB, %s\n",
		info.Identifier, info.MediaName, float64(info.SizeBytes)/1e9, info.Protocol)
	if confirm != nil && !confirm() {
		return fmt.Errorf("flash aborted by user")
	}
	if _, err := run(ctx, "diskutil", "unmountDisk", "/dev/"+info.Identifier); err != nil {
		return fmt.Errorf("unmount %s: %w", info.Identifier, err)
	}
	raw := "/dev/r" + info.Identifier // raw device (/dev/rdiskN) — derived from the validated whole-disk id
	// Writing the raw disk device requires root; dd via sudo (the operator is
	// prompted for their password unless sudo is already cached).
	if _, err := run(ctx, "sudo", "dd", "if="+imagePath, "of="+raw, "bs=4m"); err != nil {
		return fmt.Errorf("dd to %s: %w", raw, err)
	}
	_, _ = run(ctx, "diskutil", "eject", "/dev/"+info.Identifier)
	return nil
}
