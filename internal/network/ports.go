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

package network

import (
	"fmt"
	"net"
)

// CheckPortAvailable returns true if the given TCP port is free on localhost.
func CheckPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// GetFreePort asks the OS for an available TCP port by binding to port 0.
func GetFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find free port: %w", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// ResolvePort checks if the desired port is available. If not, it finds a free
// port and returns it along with a boolean indicating whether a shift occurred.
// The warning message is returned for the caller to display.
func ResolvePort(desired int) (actual int, shifted bool, warning string) {
	if CheckPortAvailable(desired) {
		return desired, false, ""
	}

	newPort, err := GetFreePort()
	if err != nil {
		// Fallback: just return the desired port and let the caller handle the error
		return desired, false, ""
	}

	warning = fmt.Sprintf(
		"⚠️  Port %d is already in use — auto-shifted to port %d.\n"+
			"   If your application hardcodes port %d (e.g., DATABASE_URL=...:%d),\n"+
			"   it will NOT connect. Use the devx-injected environment variables\n"+
			"   ($PORT, $DB_PORT, $DATABASE_URL) instead of static values.",
		desired, newPort, desired, desired,
	)

	return newPort, true, warning
}
