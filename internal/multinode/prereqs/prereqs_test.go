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

package prereqs

import (
	"strings"
	"testing"
)

func TestSocketVMNetBridgedPlist(t *testing.T) {
	p := socketVMNetBridgedPlist(
		"/opt/socket_vmnet/bin/socket_vmnet", "en0",
		"/opt/homebrew/var/run/socket_vmnet", "/opt/homebrew/var/log/socket_vmnet")
	for _, want := range []string{
		"<string>/opt/socket_vmnet/bin/socket_vmnet</string>",
		"<string>--vmnet-mode=bridged</string>",
		"<string>--vmnet-interface=en0</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",        // retry past a transient boot-time VMNET_FAILURE
		"<key>SuccessfulExit</key>",   // ...restart only on non-zero exit
		"<key>ThrottleInterval</key>", // ...without a tight crash loop
	} {
		if !strings.Contains(p, want) {
			t.Errorf("socket_vmnet bridged plist missing %q", want)
		}
	}
}
