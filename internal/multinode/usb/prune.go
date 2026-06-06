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

package usb

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/k3s"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// PruneOptions configures the ephemeral-node reaper.
type PruneOptions struct {
	TTL    time.Duration // remove NotReady ephemeral nodes whose Ready transition is older than this
	DryRun bool
}

// PruneResult reports what the reaper found and did (JSON-friendly).
type PruneResult struct {
	DryRun  bool     `json:"dry_run"`
	Stale   []string `json:"stale"`   // nodes selected for removal
	Removed []string `json:"removed"` // nodes actually drained+deleted
}

// Prune removes ephemeral USB nodes that have gone NotReady (e.g. the laptop was
// unplugged) and stayed that way past the TTL — the disposable-node cleanup that
// keeps `kubectl get nodes` free of ghosts. It only ever touches nodes carrying
// the EphemeralLabel, so permanent nodes are never reaped.
func Prune(ctx context.Context, cfg *config.Config, opts PruneOptions) (*PruneResult, error) {
	if opts.TTL <= 0 {
		opts.TTL = 30 * time.Minute
	}

	// Find a healthy server to query and act through.
	var serverNode *config.NodeConfig
	var mgr *k3s.Manager
	var nodesJSON string
	for _, n := range cfg.ServerNodes() {
		n := n
		runner := util.NewRunner(n)
		m := k3s.NewManagerWithVM(runner, n.GetVMName())
		if installed, _ := m.IsInstalled(ctx); !installed {
			continue
		}
		out, err := runner.LimaShellSudo(ctx, n.GetVMName(), "k3s kubectl get nodes -o json")
		if err != nil {
			continue
		}
		serverNode = &n
		mgr = m
		nodesJSON = out
		break
	}
	if serverNode == nil {
		return nil, fmt.Errorf("no healthy server node found to prune against")
	}

	stale, err := selectStaleEphemeral([]byte(nodesJSON), opts.TTL, time.Now())
	if err != nil {
		return nil, fmt.Errorf("selecting stale nodes: %w", err)
	}

	result := &PruneResult{DryRun: opts.DryRun, Stale: stale}
	if opts.DryRun {
		return result, nil
	}

	for _, name := range stale {
		if err := mgr.DrainNode(ctx, name); err != nil {
			// Drain of an already-gone node often fails; deletion is what matters.
			fmt.Printf("  [prune] drain %s failed (continuing): %v\n", name, err)
		}
		if err := mgr.DeleteNode(ctx, name); err != nil {
			return result, fmt.Errorf("deleting node %s: %w", name, err)
		}
		result.Removed = append(result.Removed, name)
	}
	return result, nil
}

// k8sNodeList is the minimal shape of `kubectl get nodes -o json` we need.
type k8sNodeList struct {
	Items []struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Status struct {
			Conditions []struct {
				Type               string    `json:"type"`
				Status             string    `json:"status"`
				LastTransitionTime time.Time `json:"lastTransitionTime"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

// selectStaleEphemeral returns the names of nodes that (a) carry the ephemeral
// label, (b) have a Ready condition that is not "True", and (c) transitioned out
// of Ready more than ttl ago. It is pure so it can be unit-tested with fixtures.
func selectStaleEphemeral(nodesJSON []byte, ttl time.Duration, now time.Time) ([]string, error) {
	var list k8sNodeList
	if err := json.Unmarshal(nodesJSON, &list); err != nil {
		return nil, err
	}
	var stale []string
	for _, item := range list.Items {
		if item.Metadata.Labels[EphemeralLabel] != "true" {
			continue
		}
		for _, c := range item.Status.Conditions {
			if c.Type != "Ready" {
				continue
			}
			if c.Status == "True" {
				break
			}
			if now.Sub(c.LastTransitionTime) > ttl {
				stale = append(stale, item.Metadata.Name)
			}
			break
		}
	}
	sort.Strings(stale)
	return stale, nil
}
