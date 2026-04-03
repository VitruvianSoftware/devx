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
	defer ln.Close()

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
	defer ln.Close()

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
