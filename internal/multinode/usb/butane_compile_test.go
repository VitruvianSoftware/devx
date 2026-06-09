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
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFCOSButane_Compiles renders the FCOS Butane for representative specs and
// compiles it with the real `butane --strict`, catching malformed units/files a
// substring assertion would miss. Gated on butane being installed (like the
// shellcheck test) so it skips where butane is absent.
func TestFCOSButane_Compiles(t *testing.T) {
	if _, err := exec.LookPath("butane"); err != nil {
		t.Skip("butane not in PATH — skipping")
	}
	specs := map[string]JoinSpec{
		"tailscale_ephemeral": agentSpec(),
		"lan_only": func() JoinSpec {
			s := agentSpec()
			s.TailscaleAuthKey = ""
			s.TailnetServerURL = ""
			return s
		}(),
		"install": func() JoinSpec {
			s := agentSpec()
			s.Mode = ModeInstall
			s.Ephemeral = false
			return s
		}(),
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			script, err := RenderJoinScript(spec)
			if err != nil {
				t.Fatal(err)
			}
			f := filepath.Join(t.TempDir(), "devx.bu")
			if err := os.WriteFile(f, []byte(fcosButane(spec, script)), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("butane", "--strict", f).CombinedOutput(); err != nil {
				t.Errorf("butane --strict rejected the rendered FCOS config:\n%s", out)
			}
		})
	}
}
