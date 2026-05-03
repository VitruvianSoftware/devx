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
	"net"
	"testing"
)

func TestCheckPortAvailable_FreePort(t *testing.T) {
	// Port 0 trick to get a free port, then check it after release
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port for test: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	if !CheckPortAvailable(port) {
		t.Errorf("expected port %d to be available after closing listener", port)
	}
}

func TestCheckPortAvailable_OccupiedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer func() { _ = ln.Close() }() 

	port := ln.Addr().(*net.TCPAddr).Port
	if CheckPortAvailable(port) {
		t.Errorf("expected port %d to be unavailable while listener is active", port)
	}
}

func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	if err != nil {
		t.Fatalf("GetFreePort() failed: %v", err)
	}
	if port <= 0 {
		t.Errorf("expected positive port number, got %d", port)
	}

	// The returned port should be available (we released it)
	if !CheckPortAvailable(port) {
		t.Errorf("expected port %d from GetFreePort to be available", port)
	}
}

func TestResolvePort_NoShift(t *testing.T) {
	// Get a free port
	freePort, err := GetFreePort()
	if err != nil {
		t.Fatalf("GetFreePort() failed: %v", err)
	}

	actual, shifted, warning := ResolvePort(freePort)
	if shifted {
		t.Error("expected no shift for a free port")
	}
	if actual != freePort {
		t.Errorf("expected port %d, got %d", freePort, actual)
	}
	if warning != "" {
		t.Errorf("expected empty warning, got: %s", warning)
	}
}

func TestResolvePort_WithShift(t *testing.T) {
	// Occupy a port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer func() { _ = ln.Close() }() 

	occupiedPort := ln.Addr().(*net.TCPAddr).Port

	actual, shifted, warning := ResolvePort(occupiedPort)
	if !shifted {
		t.Error("expected port to be shifted when occupied")
	}
	if actual == occupiedPort {
		t.Errorf("expected different port, got same %d", actual)
	}
	if warning == "" {
		t.Error("expected warning message when port is shifted")
	}

	// The new port should be valid
	if actual <= 0 {
		t.Errorf("expected positive shifted port, got %d", actual)
	}
}
