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
	"io"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/VitruvianSoftware/devx/internal/devxerr"
)

// TunnelConfig defines parameters for establishing the Yamux tunnel.
type TunnelConfig struct {
	Kubeconfig  string
	Context     string
	Namespace   string
	AgentPod    string
	ControlPort int // Agent's Yamux control port (4200)
	LocalPort   int // Developer's local app port
}

// Tunnel manages the kubectl port-forward subprocess and Yamux client session.
type Tunnel struct {
	cfg     TunnelConfig
	pfCmd   *exec.Cmd
	session *yamux.Session
	mu      sync.Mutex
	done    chan struct{}
}

// NewTunnel creates a new Tunnel with the given config.
func NewTunnel(cfg TunnelConfig) *Tunnel {
	return &Tunnel{
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

// Start establishes the kubectl port-forward and Yamux client session.
// It then starts proxying inbound Yamux streams to localhost:<LocalPort>.
// Returns after the first Yamux handshake succeeds. Blocks on Accept in a goroutine.
func (t *Tunnel) Start(ctx context.Context) error {
	// Find a free local port for the port-forward
	pfLocalPort, err := findFreePort()
	if err != nil {
		return fmt.Errorf("finding free port for tunnel: %w", err)
	}

	// Start kubectl port-forward
	pfArgs := []string{"port-forward", t.cfg.AgentPod,
		fmt.Sprintf("%d:%d", pfLocalPort, t.cfg.ControlPort),
		"-n", t.cfg.Namespace}
	if t.cfg.Kubeconfig != "" {
		pfArgs = append(pfArgs, "--kubeconfig", t.cfg.Kubeconfig)
	}
	if t.cfg.Context != "" {
		pfArgs = append(pfArgs, "--context", t.cfg.Context)
	}

	t.pfCmd = exec.CommandContext(ctx, "kubectl", pfArgs...)
	if err := t.pfCmd.Start(); err != nil {
		return devxerr.New(devxerr.CodeBridgeTunnelFailed,
			"failed to start kubectl port-forward for tunnel", err)
	}

	// Wait for port-forward to be ready (poll the local port)
	if err := waitForPort(pfLocalPort, 30*time.Second); err != nil {
		t.Stop()
		return devxerr.New(devxerr.CodeBridgeTunnelFailed,
			"kubectl port-forward did not become ready within 30s", err)
	}

	// Dial the local forwarded port to reach the agent
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", pfLocalPort), 10*time.Second)
	if err != nil {
		t.Stop()
		return devxerr.New(devxerr.CodeBridgeTunnelFailed,
			"failed to connect to agent via port-forward", err)
	}

	// Create Yamux client session
	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.EnableKeepAlive = true
	yamuxCfg.KeepAliveInterval = 10 * time.Second
	yamuxCfg.ConnectionWriteTimeout = 30 * time.Second

	t.session, err = yamux.Client(conn, yamuxCfg)
	if err != nil {
		_ = conn.Close()
		t.Stop()
		return devxerr.New(devxerr.CodeBridgeTunnelFailed,
			"failed to establish Yamux session with agent", err)
	}

	// Start accepting inbound streams in a goroutine
	go t.acceptStreams(ctx)

	return nil
}

// acceptStreams accepts Yamux streams from the agent and proxies them to localhost.
func (t *Tunnel) acceptStreams(ctx context.Context) {
	defer close(t.done)

	for {
		stream, err := t.session.AcceptStream()
		if err != nil {
			// Session closed or context cancelled
			select {
			case <-ctx.Done():
				return
			default:
				return // session closed
			}
		}

		// Proxy this stream to the local application
		go t.proxyStream(stream)
	}
}

// proxyStream connects a Yamux stream to localhost:<LocalPort> and copies bytes bidirectionally.
func (t *Tunnel) proxyStream(stream *yamux.Stream) {
	defer func() { _ = stream.Close() }() 

	localConn, err := net.DialTimeout("tcp",
		fmt.Sprintf("127.0.0.1:%d", t.cfg.LocalPort), 5*time.Second)
	if err != nil {
		// Local app not listening — close the stream
		return
	}
	defer func() { _ = localConn.Close() }() 

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(localConn, stream)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(stream, localConn)
	}()

	wg.Wait()
}

// Stop gracefully closes the Yamux session and kills the port-forward subprocess.
func (t *Tunnel) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.session != nil {
		_ = t.session.Close()
		t.session = nil
	}

	if t.pfCmd != nil && t.pfCmd.Process != nil {
		_ = t.pfCmd.Process.Kill()
		_ = t.pfCmd.Wait()
		t.pfCmd = nil
	}
}

// Healthy returns true if the Yamux session is still alive.
func (t *Tunnel) Healthy() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.session == nil {
		return false
	}
	return !t.session.IsClosed()
}

// Done returns a channel that is closed when the tunnel stops accepting streams.
func (t *Tunnel) Done() <-chan struct{} {
	return t.done
}

// findFreePort returns an available local TCP port.
func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }() 
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitForPort polls until a TCP port is accepting connections.
func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("port %d did not become available within %s", port, timeout)
}
