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

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/network"
)

// serviceForward is a discovered Service port to forward to localhost.
type serviceForward struct {
	Service string
	Port    int
}

// parseServiceForwards extracts (service, port) pairs from `kubectl get svc -o json`
// output. Pure function, separated for unit testing.
func parseServiceForwards(jsonOut []byte) ([]serviceForward, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Ports []struct {
					Port int `json:"port"`
				} `json:"ports"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(jsonOut, &list); err != nil {
		return nil, err
	}
	var fwds []serviceForward
	for _, it := range list.Items {
		for _, p := range it.Spec.Ports {
			if p.Port > 0 {
				fwds = append(fwds, serviceForward{Service: it.Metadata.Name, Port: p.Port})
			}
		}
	}
	return fwds, nil
}

// activeForward records a started port-forward — its resolved local port and the
// Service/port it targets — returned for visibility and tests.
type activeForward struct {
	Local   int
	Service string
	Port    int
}

// startPortForwards discovers the Services in ns (skaffold-style automatic
// discovery) and starts a background `kubectl port-forward` for each port, mapping
// it to a conflict-resolved local port. It returns the active forwards plus a cancel
// func that stops every forward (the DAG calls it on shutdown). devx's richer
// `bridge` (intercept/steal) is unchanged — this covers the simple "reach my
// deployed service locally" case.
func startPortForwards(parent context.Context, kubeconfig, kctx, ns string) ([]activeForward, context.CancelFunc, error) {
	out, err := runKubectl(parent, kubectlArgs(kubeconfig, kctx, "get", "svc", "-n", ns, "-o", "json")...)
	if err != nil {
		return nil, nil, fmt.Errorf("discovering services for port-forward: %w\n%s", err, strings.TrimSpace(out))
	}
	fwds, err := parseServiceForwards([]byte(out))
	if err != nil {
		return nil, nil, fmt.Errorf("parsing services for port-forward: %w", err)
	}
	if len(fwds) == 0 {
		return nil, func() {}, nil
	}

	ctx, cancel := context.WithCancel(parent)
	var active []activeForward
	for _, f := range fwds {
		local, _, _ := network.ResolvePort(f.Port)
		cmd := exec.CommandContext(ctx, "kubectl",
			kubectlArgs(kubeconfig, kctx, "port-forward", "-n", ns, "svc/"+f.Service,
				fmt.Sprintf("%d:%d", local, f.Port))...)
		if err := cmd.Start(); err != nil {
			cancel()
			return nil, nil, fmt.Errorf("starting port-forward for svc/%s: %w", f.Service, err)
		}
		go func() { _ = cmd.Wait() }() // reap the process when it's killed on cancel
		active = append(active, activeForward{Local: local, Service: f.Service, Port: f.Port})
		fmt.Printf("  🔌 port-forward: localhost:%d → svc/%s:%d (ns/%s)\n", local, f.Service, f.Port, ns)
	}
	return active, cancel, nil
}
