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

// Package orchestrator — bridge_node.go implements the DAG executor's handler
// for RuntimeBridge services. This is the integration glue between the DAG
// and the internal/bridge/ package. (Idea 46.3)
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/VitruvianSoftware/devx/internal/bridge"
)

// BridgeNodeState holds the runtime state of a running bridge node, used for teardown.
type BridgeNodeState struct {
	PortForward *bridge.PortForward  // non-nil for connect mode
	Tunnel      *bridge.Tunnel       // non-nil for intercept mode
	AgentInfo   *bridge.AgentInfo    // non-nil for intercept mode
	SvcState    *bridge.ServiceState // non-nil for intercept mode
	SessionID   string               // set for intercept mode
	Kubeconfig  string               // for cleanup kubectl calls
	Context     string               // for cleanup kubectl calls
	Namespace   string               // for cleanup kubectl calls
}

// Cleanup tears down bridge state in the correct order:
// 1. Stop tunnel (disconnect Yamux session)
// 2. Restore service selector (un-redirect traffic)
// 3. Remove agent pod
// 4. Stop port-forward subprocess
// 5. Remove session entry
func (s *BridgeNodeState) Cleanup() {
	if s == nil {
		return
	}

	// Intercept cleanup: restore selector BEFORE removing agent
	if s.SvcState != nil {
		if err := bridge.RestoreServiceSelector(s.Kubeconfig, s.Context, s.SvcState); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to restore selector for %s: %v (agent will auto-restore)\n", s.SvcState.Name, err)
		}
	}
	if s.Tunnel != nil {
		s.Tunnel.Stop()
	}
	if s.AgentInfo != nil && s.SessionID != "" {
		_ = bridge.RemoveAgent(s.Kubeconfig, s.Context, s.Namespace, s.SessionID)
	}
	if s.PortForward != nil {
		s.PortForward.Stop()
	}

	// Clean up session file entry
	if s.SessionID != "" {
		removeDAGSessionEntry(s.SessionID)
	}
}

// startBridgeNode dispatches to connect or intercept based on bridge mode.
// It also validates kubectl availability lazily (only on first bridge node).
func startBridgeNode(ctx context.Context, n *Node) error {
	if n.BridgeConfig == nil {
		return fmt.Errorf("bridge node %q has no BridgeConfig", n.Name)
	}

	cfg := n.BridgeConfig

	// Lazy kubectl validation — only when we actually need it (Idea 46.3 Q2 resolution)
	if _, err := bridge.ValidateKubectl(); err != nil {
		return err
	}

	kcPath, err := bridge.ResolveKubeconfig(cfg.Kubeconfig)
	if err != nil {
		return err
	}
	if err := bridge.ValidateContext(kcPath, cfg.Context); err != nil {
		return err
	}

	switch cfg.Mode {
	case BridgeModeConnect:
		return startBridgeConnect(ctx, n, kcPath, cfg)
	case BridgeModeIntercept:
		return startBridgeIntercept(ctx, n, kcPath, cfg)
	default:
		return fmt.Errorf("unknown bridge mode %q for node %q", cfg.Mode, n.Name)
	}
}

// startBridgeConnect sets up an outbound kubectl port-forward to a remote K8s service.
// pf.Start() blocks forever (runs kubectl with a retry loop), so it MUST run in a goroutine.
// Readiness is tracked via pf.State() and polled by waitForBridgeHealthy().
func startBridgeConnect(ctx context.Context, n *Node, kcPath string, cfg *BridgeNodeConfig) error {
	// Validate service exists in the cluster
	if err := bridge.ValidateService(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService); err != nil {
		return err
	}

	pf := bridge.NewPortForward(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService, cfg.RemotePort, cfg.LocalPort)

	if warning, err := pf.ResolveLocalPort(); err != nil {
		return err
	} else if warning != "" {
		fmt.Println(warning)
	}

	// Update node's resolved port for downstream dependencies
	n.Port = pf.LocalPort

	// Store state for cleanup
	n.bridgeState = &BridgeNodeState{
		PortForward: pf,
		Kubeconfig:  kcPath,
		Context:     cfg.Context,
		Namespace:   cfg.Namespace,
	}

	fmt.Printf("  🔗 Bridging %s → %s/%s:%d → localhost:%d\n",
		n.Name, cfg.Namespace, cfg.TargetService, cfg.RemotePort, pf.LocalPort)

	// CRITICAL: pf.Start() blocks forever (kubectl port-forward with retry loop).
	// We spawn it in a goroutine so the DAG tier's wg.Wait() can proceed.
	// Readiness is polled by waitForBridgeHealthy() via pf.State().
	go func() {
		_ = pf.Start(ctx)
	}()

	return nil
}

// startBridgeIntercept sets up inbound traffic interception from a remote K8s service.
//
// This has two phases:
//
//	Phase A (synchronous, finite ~10-30s):
//	  1. Inspect + validate service
//	  2. Deploy agent pod
//	  3. Patch service selector
//	  4. Establish Yamux tunnel
//
//	Phase B (goroutine, blocks forever):
//	  5. Tunnel maintenance loop (keepalives, stream proxying)
//
// When startBridgeIntercept returns nil, the intercept IS fully operational.
// No separate healthcheck is needed — the setup steps ARE the readiness signal.
func startBridgeIntercept(ctx context.Context, n *Node, kcPath string, cfg *BridgeNodeConfig) error {
	// 1. Inspect + validate
	info, err := bridge.InspectService(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService)
	if err != nil {
		return err
	}
	if err := bridge.ValidateInterceptable(info); err != nil {
		return err
	}

	// 2. Check for existing intercept conflict
	if err := bridge.CheckInterceptConflict(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService); err != nil {
		return err
	}

	// 3. Deploy agent
	sessionID := uuid.New().String()[:8]
	agentImage := cfg.AgentImage
	if agentImage == "" {
		agentImage = bridge.AgentImageDefault
	}
	agentCfg := bridge.AgentConfig{
		Kubeconfig:       kcPath,
		Context:          cfg.Context,
		Namespace:        cfg.Namespace,
		TargetService:    cfg.TargetService,
		InterceptPort:    cfg.RemotePort,
		ServicePorts:     info.Ports,
		OriginalSelector: info.Selector,
		AgentImage:       agentImage,
		SessionID:        sessionID,
	}

	fmt.Printf("  🔀 Deploying intercept agent for %s/%s...\n", cfg.Namespace, cfg.TargetService)

	agentInfo, err := bridge.DeployAgent(agentCfg)
	if err != nil {
		return err
	}

	// 4. Patch selector
	agentSelector := map[string]string{
		"devx-bridge":         "agent",
		"devx-bridge-session": sessionID,
	}
	svcState, err := bridge.PatchServiceSelector(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService, agentSelector, sessionID)
	if err != nil {
		_ = bridge.RemoveAgent(kcPath, cfg.Context, cfg.Namespace, sessionID)
		return err
	}

	// 5. Establish Yamux tunnel (Start returns after handshake; maintenance runs in goroutine)
	tunnel := bridge.NewTunnel(bridge.TunnelConfig{
		Kubeconfig:  kcPath,
		Context:     cfg.Context,
		Namespace:   cfg.Namespace,
		AgentPod:    agentInfo.PodName,
		ControlPort: agentInfo.ControlPort,
		LocalPort:   cfg.LocalPort,
	})
	if err := tunnel.Start(ctx); err != nil {
		_ = bridge.RestoreServiceSelector(kcPath, cfg.Context, svcState)
		_ = bridge.RemoveAgent(kcPath, cfg.Context, cfg.Namespace, sessionID)
		return err
	}

	// Store state for DAG cleanup
	n.bridgeState = &BridgeNodeState{
		Tunnel:     tunnel,
		AgentInfo:  agentInfo,
		SvcState:   svcState,
		SessionID:  sessionID,
		Kubeconfig: kcPath,
		Context:    cfg.Context,
		Namespace:  cfg.Namespace,
	}

	fmt.Printf("  🔀 Intercepting %s/%s:%d → localhost:%d (steal)\n",
		cfg.Namespace, cfg.TargetService, cfg.RemotePort, cfg.LocalPort)

	// 6. Persist session with DAG origin
	interceptEntry := bridge.InterceptEntry{
		Service:          cfg.TargetService,
		Namespace:        cfg.Namespace,
		TargetPort:       cfg.RemotePort,
		LocalPort:        cfg.LocalPort,
		Mode:             "steal",
		AgentPod:         agentInfo.PodName,
		SessionID:        sessionID,
		OriginalSelector: info.Selector,
		StartedAt:        time.Now(),
		Origin:           "dag",
	}
	session, _ := bridge.LoadSession()
	if session == nil {
		session = &bridge.Session{
			Kubeconfig: kcPath,
			Context:    cfg.Context,
			StartedAt:  time.Now(),
		}
	}
	session.Intercepts = append(session.Intercepts, interceptEntry)
	_ = bridge.SaveSession(session)

	// Returning nil = intercept is fully operational.
	return nil
}

// waitForBridgeHealthy polls pf.State() for a connect bridge node until healthy or timeout.
func waitForBridgeHealthy(ctx context.Context, n *Node) error {
	if n.bridgeState == nil || n.bridgeState.PortForward == nil {
		return fmt.Errorf("bridge node %q has no port-forward state", n.Name)
	}

	pf := n.bridgeState.PortForward

	// Default timeout for bridge health — matches the PortForward internal health probe
	timeout := 30 * time.Second
	if n.Healthcheck.Timeout > 0 {
		timeout = n.Healthcheck.Timeout
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			lastErr := pf.LastError()
			if lastErr != nil {
				return fmt.Errorf("bridge unhealthy after %s: %w", timeout, lastErr)
			}
			return fmt.Errorf("bridge unhealthy after %s (state: %s)", timeout, pf.State())
		case <-ticker.C:
			if pf.State() == bridge.StateHealthy {
				return nil
			}
			if pf.State() == bridge.StateFailed {
				return fmt.Errorf("bridge port-forward failed: %v", pf.LastError())
			}
		}
	}
}

// removeDAGSessionEntry removes intercepts and entries with the given session ID from the session file.
func removeDAGSessionEntry(sessionID string) {
	session, err := bridge.LoadSession()
	if err != nil || session == nil {
		return
	}

	var remainingEntries []bridge.SessionEntry
	for _, e := range session.Entries {
		if e.Origin == "dag" && e.Service == sessionID {
			continue
		}
		remainingEntries = append(remainingEntries, e)
	}
	session.Entries = remainingEntries

	var remainingIntercepts []bridge.InterceptEntry
	for _, ic := range session.Intercepts {
		if ic.SessionID == sessionID {
			continue
		}
		remainingIntercepts = append(remainingIntercepts, ic)
	}
	session.Intercepts = remainingIntercepts

	if len(session.Entries) == 0 && len(session.Intercepts) == 0 {
		_ = bridge.ClearSession()
	} else {
		_ = bridge.SaveSession(session)
	}
}

// GenerateBridgeEnvVars returns the BRIDGE_*_URL env vars for all bridge services in the DAG.
func (d *DAG) GenerateBridgeEnvVars() map[string]string {
	envs := make(map[string]string)
	for _, n := range d.Nodes {
		if n.Runtime != RuntimeBridge || n.bridgeState == nil {
			continue
		}
		if n.BridgeConfig == nil {
			continue
		}

		port := n.Port
		if port == 0 && n.BridgeConfig.LocalPort > 0 {
			port = n.BridgeConfig.LocalPort
		}

		// Generate BRIDGE_<SERVICE_NAME>_URL=http://localhost:<port>
		envKey := fmt.Sprintf("BRIDGE_%s_URL", toEnvName(n.BridgeConfig.TargetService))
		envs[envKey] = fmt.Sprintf("http://localhost:%d", port)
	}
	return envs
}

// WriteBridgeEnvFile writes the BRIDGE_*_URL env vars to ~/.devx/bridge.env.
func (d *DAG) WriteBridgeEnvFile() error {
	envs := d.GenerateBridgeEnvVars()
	if len(envs) == 0 {
		return nil
	}

	envPath, err := bridge.EnvPath()
	if err != nil {
		return err
	}

	var content string
	for k, v := range envs {
		content += fmt.Sprintf("%s=%s\n", k, v)
	}

	return os.WriteFile(envPath, []byte(content), 0o644)
}

// toEnvName converts a service name to an environment variable segment.
// e.g. "payments-api" → "PAYMENTS_API"
func toEnvName(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if c == '-' || c == '.' {
			result = append(result, '_')
		} else if c >= 'a' && c <= 'z' {
			result = append(result, c-32)
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

// BridgeServicesJSON returns a JSON-serializable summary of bridge services for --dry-run.
func (d *DAG) BridgeServicesJSON() ([]byte, error) {
	type bridgeSummary struct {
		Name    string `json:"name"`
		Mode    string `json:"mode"`
		Service string `json:"target_service"`
		NS      string `json:"namespace"`
		Remote  int    `json:"remote_port"`
		Local   int    `json:"local_port"`
	}
	var summaries []bridgeSummary
	for _, n := range d.Nodes {
		if n.Runtime != RuntimeBridge || n.BridgeConfig == nil {
			continue
		}
		summaries = append(summaries, bridgeSummary{
			Name:    n.Name,
			Mode:    string(n.BridgeConfig.Mode),
			Service: n.BridgeConfig.TargetService,
			NS:      n.BridgeConfig.Namespace,
			Remote:  n.BridgeConfig.RemotePort,
			Local:   n.BridgeConfig.LocalPort,
		})
	}
	return json.MarshalIndent(summaries, "", "  ")
}
