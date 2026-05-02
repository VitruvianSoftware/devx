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

package bridge

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/network"
)

// PortForwardState represents the health of a single port-forward.
type PortForwardState int

const (
	StateStarting PortForwardState = iota
	StateHealthy
	StateFailed
	StateStopped
)

func (s PortForwardState) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateHealthy:
		return "healthy"
	case StateFailed:
		return "failed"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// PortForward manages a single kubectl port-forward subprocess.
type PortForward struct {
	Service    string // K8s service name
	Namespace  string
	RemotePort int
	LocalPort  int

	kubeconfig string
	kubeCtx    string
	cmd        *exec.Cmd
	cancel     context.CancelFunc

	mu       sync.RWMutex
	state    PortForwardState
	lastErr  error
	retries  int
	stateC   chan PortForwardState
}

const maxRetries = 3

// NewPortForward creates a new port-forward manager.
func NewPortForward(kubeconfig, kubeCtx, namespace, service string, remotePort, localPort int) *PortForward {
	return &PortForward{
		Service:    service,
		Namespace:  namespace,
		RemotePort: remotePort,
		LocalPort:  localPort,
		kubeconfig: kubeconfig,
		kubeCtx:    kubeCtx,
		state:      StateStarting,
		stateC:     make(chan PortForwardState, 8),
	}
}

// ResolveLocalPort acquires a free local port if LocalPort is 0, or resolves
// port collisions if the desired port is in use.
func (pf *PortForward) ResolveLocalPort() (string, error) {
	if pf.LocalPort == 0 {
		port, err := network.GetFreePort()
		if err != nil {
			return "", fmt.Errorf("auto-assigning port for %s: %w", pf.Service, err)
		}
		pf.LocalPort = port
		return "", nil
	}

	actual, shifted, warning := network.ResolvePort(pf.LocalPort)
	pf.LocalPort = actual
	if shifted {
		return warning, nil
	}
	return "", nil
}

// Start launches the kubectl port-forward subprocess. It blocks until the
// context is cancelled or the port-forward fails after all retries.
func (pf *PortForward) Start(ctx context.Context) error {
	for {
		pf.mu.Lock()
		pf.retries++
		pf.mu.Unlock()

		err := pf.run(ctx)
		if err == nil || ctx.Err() != nil {
			// Clean shutdown or context cancelled
			pf.setState(StateStopped)
			return nil
		}

		pf.mu.RLock()
		retries := pf.retries
		pf.mu.RUnlock()

		if retries >= maxRetries {
			pf.mu.Lock()
			pf.lastErr = err
			pf.mu.Unlock()
			pf.setState(StateFailed)
			return devxerr.New(devxerr.CodeBridgePortForwardFailed,
				fmt.Sprintf("port-forward %s/%s failed after %d retries", pf.Namespace, pf.Service, maxRetries), err)
		}

		// Exponential backoff: 1s, 2s, 4s
		backoff := time.Duration(1<<uint(retries-1)) * time.Second
		pf.setState(StateStarting)

		select {
		case <-time.After(backoff):
			// retry
		case <-ctx.Done():
			pf.setState(StateStopped)
			return nil
		}
	}
}

// run executes a single kubectl port-forward subprocess.
func (pf *PortForward) run(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)
	pf.cancel = cancel

	args := []string{
		"port-forward",
		fmt.Sprintf("svc/%s", pf.Service),
		fmt.Sprintf("%d:%d", pf.LocalPort, pf.RemotePort),
		"-n", pf.Namespace,
		"--kubeconfig", pf.kubeconfig,
	}
	if pf.kubeCtx != "" {
		args = append(args, "--context", pf.kubeCtx)
	}

	pf.cmd = exec.CommandContext(childCtx, "kubectl", args...)

	// Capture stderr for error reporting
	var stderr strings.Builder
	pf.cmd.Stderr = &stderr

	if err := pf.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("starting port-forward: %w", err)
	}

	// Wait briefly for the port to become healthy
	go func() {
		for i := 0; i < 20; i++ {
			if childCtx.Err() != nil {
				return
			}
			conn, err := net.DialTimeout("tcp",
				fmt.Sprintf("127.0.0.1:%d", pf.LocalPort), 200*time.Millisecond)
			if err == nil {
				conn.Close()
				pf.setState(StateHealthy)
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()

	err := pf.cmd.Wait()
	cancel()

	if err != nil && childCtx.Err() == nil {
		// Subprocess died unexpectedly
		return fmt.Errorf("port-forward exited: %s (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Stop gracefully terminates the port-forward subprocess.
func (pf *PortForward) Stop() {
	if pf.cancel != nil {
		pf.cancel()
	}
	pf.setState(StateStopped)
}

// State returns the current state of this port-forward.
func (pf *PortForward) State() PortForwardState {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.state
}

// StateChannel returns a channel that receives state transitions.
func (pf *PortForward) StateChannel() <-chan PortForwardState {
	return pf.stateC
}

// LastError returns the last error that caused a retry or failure.
func (pf *PortForward) LastError() error {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.lastErr
}

func (pf *PortForward) setState(s PortForwardState) {
	pf.mu.Lock()
	pf.state = s
	pf.mu.Unlock()

	select {
	case pf.stateC <- s:
	default:
		// Non-blocking send — drop if no one is listening
	}
}

// LocalAddr returns the local address string (e.g. "127.0.0.1:9501").
func (pf *PortForward) LocalAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", pf.LocalPort)
}
