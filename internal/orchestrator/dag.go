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

// Package orchestrator implements a DAG-based service dependency graph for
// ordered startup of databases, mock servers, and developer applications.
package orchestrator

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VitruvianSoftware/devx/internal/logs"
	"github.com/VitruvianSoftware/devx/internal/network"
)

// Runtime determines where a service executes.
type Runtime string

const (
	RuntimeHost       Runtime = "host"
	RuntimeContainer  Runtime = "container"
	RuntimeKubernetes Runtime = "kubernetes"
	RuntimeCloud      Runtime = "cloud"
	RuntimeBridge     Runtime = "bridge" // Idea 46.3: hybrid bridge services
)

// BridgeMode distinguishes between outbound and inbound bridge operations.
type BridgeMode string

const (
	BridgeModeConnect   BridgeMode = "connect"   // Outbound: kubectl port-forward
	BridgeModeIntercept BridgeMode = "intercept" // Inbound: agent + yamux tunnel
)

// BridgeNodeConfig holds bridge-specific configuration for a DAG node.
type BridgeNodeConfig struct {
	Kubeconfig    string
	Context       string
	Namespace     string
	TargetService string
	RemotePort    int
	LocalPort     int
	AgentImage    string
	Mode          BridgeMode
	InterceptMode string // "steal" or "mirror"
}

// HealthcheckConfig defines how to verify a service is ready.
type HealthcheckConfig struct {
	// HTTP endpoint to poll (e.g., "http://localhost:8080/health")
	HTTP string `yaml:"http"`
	// TCP address to probe (e.g., "localhost:5432")
	TCP string `yaml:"tcp"`
	// Interval between checks
	Interval time.Duration `yaml:"interval"`
	// Timeout for each check attempt
	Timeout time.Duration `yaml:"timeout"`
	// Number of consecutive successes required
	Retries int `yaml:"retries"`
}

// DependsOnEntry references another node and a readiness condition.
type DependsOnEntry struct {
	Name      string `yaml:"name"`
	Condition string `yaml:"condition"` // "service_healthy" or "service_started"
}

// ServiceConfig defines a developer application in devx.yaml.
type ServiceConfig struct {
	Name        string            `yaml:"name"`
	Runtime     Runtime           `yaml:"runtime"`
	Command     []string          `yaml:"command"`
	DependsOn   []DependsOnEntry  `yaml:"depends_on"`
	Healthcheck HealthcheckConfig `yaml:"healthcheck"`
	Port        int               `yaml:"port"`
	Env         map[string]string `yaml:"env"`
}

// NodeType categorises a DAG node.
type NodeType int

const (
	NodeDatabase NodeType = iota
	NodeMock
	NodeService
)

// Node is a single unit of work in the DAG.
type Node struct {
	Name        string
	Type        NodeType
	DependsOn   []string // names of nodes this depends on
	Healthcheck HealthcheckConfig
	Port        int    // resolved port (after conflict resolution)
	Runtime     Runtime
	Command     []string
	Env         map[string]string
	Dir         string // Working directory for host process execution (set by include resolver for multirepo)

	// Bridge-specific fields (Idea 46.3)
	BridgeMode   BridgeMode       // for RuntimeBridge nodes
	BridgeConfig *BridgeNodeConfig // bridge-specific parameters

	// Runtime state
	process     *exec.Cmd
	cancel      context.CancelFunc
	bridgeState *BridgeNodeState // runtime state for bridge cleanup
}

// DAG is a directed acyclic graph of nodes.
type DAG struct {
	Nodes map[string]*Node
	mu    sync.Mutex
}

// NewDAG creates an empty DAG.
func NewDAG() *DAG {
	return &DAG{Nodes: make(map[string]*Node)}
}

// AddNode registers a node.
func (d *DAG) AddNode(n *Node) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.Nodes[n.Name]; exists {
		return fmt.Errorf("duplicate node: %s", n.Name)
	}
	d.Nodes[n.Name] = n
	return nil
}

// Validate checks for missing dependencies and cycles.
func (d *DAG) Validate() error {
	for name, node := range d.Nodes {
		for _, dep := range node.DependsOn {
			if _, exists := d.Nodes[dep]; !exists {
				return fmt.Errorf("node %q depends on unknown node %q", name, dep)
			}
		}
	}

	// Cycle detection via topological sort
	_, err := d.TopologicalSort()
	return err
}

// TopologicalSort returns execution tiers: each tier contains nodes that
// can be started in parallel once all nodes in previous tiers are healthy.
func (d *DAG) TopologicalSort() ([][]string, error) {
	inDegree := make(map[string]int)
	for name := range d.Nodes {
		inDegree[name] = 0
	}
	for _, node := range d.Nodes {
		for _, dep := range node.DependsOn {
			inDegree[node.Name]++
			_ = dep // edge from dep -> node.Name
		}
	}

	// Kahn's algorithm
	var tiers [][]string
	remaining := make(map[string]bool)
	for name := range d.Nodes {
		remaining[name] = true
	}

	for len(remaining) > 0 {
		var tier []string
		for name := range remaining {
			if inDegree[name] == 0 {
				tier = append(tier, name)
			}
		}

		if len(tier) == 0 {
			// All remaining have incoming edges — cycle detected
			var cycleNodes []string
			for name := range remaining {
				cycleNodes = append(cycleNodes, name)
			}
			sort.Strings(cycleNodes)
			return nil, fmt.Errorf("dependency cycle detected among: %s", strings.Join(cycleNodes, ", "))
		}

		// Deterministic ordering within a tier
		sort.Strings(tier)

		for _, name := range tier {
			delete(remaining, name)
			// Decrement in-degree for dependents
			for depName, depNode := range d.Nodes {
				for _, dep := range depNode.DependsOn {
					if dep == name {
						inDegree[depName]--
					}
				}
			}
		}

		tiers = append(tiers, tier)
	}

	return tiers, nil
}

// Execute starts all nodes in topological order, waiting for health checks
// between tiers. It returns a cleanup function to stop all started processes.
func (d *DAG) Execute(ctx context.Context) (cleanup func(), err error) {
	tiers, err := d.TopologicalSort()
	if err != nil {
		return nil, err
	}

	var startedNodes []*Node

	cleanupFn := func() {
		for i := len(startedNodes) - 1; i >= 0; i-- {
			n := startedNodes[i]

			// Bridge cleanup: restore selectors, remove agents, stop port-forwards
			if n.bridgeState != nil {
				n.bridgeState.Cleanup()
			}

			if n.cancel != nil {
				n.cancel()
			}
			if n.process != nil && n.process.Process != nil {
				_ = n.process.Process.Kill()
			}
		}
	}

	for tierIdx, tier := range tiers {
		fmt.Printf("\n📋 Starting tier %d: %s\n", tierIdx+1, strings.Join(tier, ", "))

		var wg sync.WaitGroup
		errCh := make(chan error, len(tier))

		for _, name := range tier {
			node := d.Nodes[name]
			wg.Add(1)

			go func(n *Node) {
				defer wg.Done()

			if n.Type == NodeService {
					switch n.Runtime {
					case RuntimeBridge:
						if err := startBridgeNode(ctx, n); err != nil {
							errCh <- fmt.Errorf("failed to start bridge service %q: %w", n.Name, err)
							return
						}
					default:
						if len(n.Command) > 0 {
							if n.Port > 0 {
								actual, shifted, warning := network.ResolvePort(n.Port)
								if shifted {
									fmt.Fprintf(os.Stderr, "\n%s\n\n", warning)
									n.Port = actual
								}
							}

							if err := startHostProcess(ctx, n); err != nil {
								errCh <- fmt.Errorf("failed to start service %q: %w", n.Name, err)
								return
							}
						}
					}
				}

				startedNodes = append(startedNodes, n)
			}(node)
		}

		wg.Wait()
		close(errCh)

		// Collect errors
		for e := range errCh {
			cleanupFn()
			return nil, e
		}

		// Wait for health checks on this tier before moving to the next tier
		for _, name := range tier {
			node := d.Nodes[name]

			// Bridge-native health path: intercept nodes are already healthy when
			// startBridgeIntercept returns nil. Connect nodes use pf.State().
			if node.Runtime == RuntimeBridge && node.bridgeState != nil {
				if node.BridgeMode == BridgeModeIntercept {
					// Intercept: returning nil from startBridgeIntercept IS the readiness signal
					fmt.Printf("  \u2705 %s is healthy (intercept active)\n", name)
					continue
				}
				if node.BridgeMode == BridgeModeConnect && node.bridgeState.PortForward != nil {
					// Connect: poll pf.State() instead of naive TCP
					fmt.Printf("  \u23f3 Waiting for %s bridge to become healthy...\n", name)
					if err := waitForBridgeHealthy(ctx, node); err != nil {
						cleanupFn()
						return nil, fmt.Errorf("bridge healthcheck failed for %q: %w", name, err)
					}
					fmt.Printf("  \u2705 %s is healthy\n", name)
					continue
				}
			}

			if node.Healthcheck.HTTP != "" || node.Healthcheck.TCP != "" {
				fmt.Printf("  \u23f3 Waiting for %s to become healthy...\n", name)
				if err := waitForHealthy(ctx, node); err != nil {
					// Idea 35: Tail crash logs inline
					if node.Type == NodeService {
						logs.TailHostCrashLogs(node.Name, 50)
					}
					cleanupFn()
					return nil, fmt.Errorf("healthcheck failed for %q: %w", name, err)
				}
				fmt.Printf("  \u2705 %s is healthy\n", name)
			}
		}
	}

	return cleanupFn, nil
}

// startHostProcess launches a native host process for a service node.
func startHostProcess(ctx context.Context, n *Node) error {
	childCtx, cancel := context.WithCancel(ctx)
	n.cancel = cancel

	cmd := exec.CommandContext(childCtx, n.Command[0], n.Command[1:]...)

	// Idea 44: Set working directory for services from included (sibling) repositories.
	// When empty, inherits the parent's CWD (existing single-project behavior).
	if n.Dir != "" {
		cmd.Dir = n.Dir
	}
	// Setup logging to ~/.devx/logs/<name>.log
	logDir := filepath.Join(os.Getenv("HOME"), ".devx", "logs")
	_ = os.MkdirAll(logDir, 0755)

	logFile, err := os.OpenFile(
		filepath.Join(logDir, n.Name+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644,
	)
	if err != nil {
		cancel()
		return fmt.Errorf("opening log file: %w", err)
	}

	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)

	// Inject environment variables
	cmd.Env = os.Environ()
	for k, v := range n.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if n.Port > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", n.Port))
	}

	n.process = cmd

	fmt.Printf("  🚀 Starting %s: %s\n", n.Name, strings.Join(n.Command, " "))

	return cmd.Start()
}

// waitForHealthy polls the healthcheck until it passes or context is cancelled.
func waitForHealthy(ctx context.Context, n *Node) error {
	hc := n.Healthcheck
	interval := hc.Interval
	if interval == 0 {
		interval = 1 * time.Second
	}
	timeout := hc.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	retries := hc.Retries
	if retries == 0 {
		retries = 1
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	consecutiveOK := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timed out after %s", timeout)
		case <-ticker.C:
			var ok bool
			if hc.HTTP != "" {
				ok = checkHTTP(hc.HTTP)
			} else if hc.TCP != "" {
				ok = checkTCP(hc.TCP)
			}

			if ok {
				consecutiveOK++
				if consecutiveOK >= retries {
					return nil
				}
			} else {
				consecutiveOK = 0
			}
		}
	}
}

func checkHTTP(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func checkTCP(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
