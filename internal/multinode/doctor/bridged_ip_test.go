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

package doctor

import "testing"

// isBridgedIP must reject Lima's user-mode (slirp) 192.168.5.0/24 — that's the
// fallback GetBridgedIP returns when no real socket_vmnet bridge (lima0) exists.
func TestIsBridgedIP(t *testing.T) {
	cases := map[string]bool{
		"192.168.86.58":  true, // real LAN bridge (socket_vmnet)
		"192.168.86.200": true,
		"10.4.0.7":       true,  // any non-user-mode address
		"192.168.5.15":   false, // Lima user-mode (slirp) — NOT a bridge
		"192.168.5.2":    false,
		"":               false,
	}
	for ip, want := range cases {
		if got := isBridgedIP(ip); got != want {
			t.Errorf("isBridgedIP(%q) = %v, want %v", ip, got, want)
		}
	}
}
