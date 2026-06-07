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
	"strings"
	"testing"
)

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
