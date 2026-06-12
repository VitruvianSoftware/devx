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
	"strings"
	"testing"
)

func TestNodeKeeperScript(t *testing.T) {
	s := nodeKeeperScript("/opt/homebrew/bin/limactl", "k8s-node", "/opt/homebrew/var/run/socket_vmnet")
	if !strings.HasPrefix(s, "#!/bin/bash") {
		t.Error("keeper script must start with a bash shebang")
	}
	for _, want := range []string{
		`LIMACTL="/opt/homebrew/bin/limactl"`,
		`VM="k8s-node"`,
		`SOCK="/opt/homebrew/var/run/socket_vmnet"`,
		`[ -S "$SOCK" ] && break`,           // wait for the socket (socket_vmnet KeepAlive heals it)
		`list "$VM" --format '{{.Status}}'`, // read VM status
		`!= "Running"`,                      // idempotent: act only if not running
		`"$LIMACTL" start "$VM"`,            // start the VM
	} {
		if !strings.Contains(s, want) {
			t.Errorf("keeper script missing %q in:\n%s", want, s)
		}
	}
}

func TestNodeKeeperPlist(t *testing.T) {
	p := nodeKeeperPlist("/Users/james/.devx/node-vm-keeper.sh", "/Users/james/.devx/node-vm-keeper.log")
	for _, want := range []string{
		"<string>" + NodeKeeperLabel + "</string>",
		"<string>/bin/bash</string>",
		"<string>/Users/james/.devx/node-vm-keeper.sh</string>",
		"<key>RunAtLoad</key>",
		"<key>StartInterval</key>",
		"<integer>120</integer>", // periodic watchdog
		"<key>StandardErrorPath</key>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("keeper plist missing %q in:\n%s", want, p)
		}
	}
	// It is a user LaunchAgent (the vz driver needs the GUI session), not a daemon.
	if strings.Contains(p, "LaunchDaemon") {
		t.Error("keeper must be a LaunchAgent, not reference LaunchDaemon")
	}
}
