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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DirSink is an ArtifactSink that writes artifacts as files under Root.
type DirSink struct {
	Root string
}

// Add implements ArtifactSink. Paths use forward slashes and are written
// relative to Root; parent directories are created as needed.
func (d DirSink) Add(a Artifact) error {
	if strings.Contains(a.Path, "..") {
		return fmt.Errorf("artifact path %q must not contain parent-directory references", a.Path)
	}
	mode := a.Mode
	if mode == 0 {
		mode = 0o644
	}
	full := filepath.Join(d.Root, filepath.FromSlash(a.Path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("creating dir for %s: %w", a.Path, err)
	}
	if err := os.WriteFile(full, a.Contents, mode); err != nil {
		return fmt.Errorf("writing %s: %w", a.Path, err)
	}
	return nil
}
