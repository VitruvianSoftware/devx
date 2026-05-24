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
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/VitruvianSoftware/devx/internal/logs"
)

// --- watch-event decoding (pure, unit-tested) ---

type podWatchEvent struct {
	Type   string `json:"type"`
	Object struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Status struct {
			InitContainerStatuses []containerStatus `json:"initContainerStatuses"`
			ContainerStatuses     []containerStatus `json:"containerStatuses"`
		} `json:"status"`
	} `json:"object"`
}

type containerStatus struct {
	Name        string `json:"name"`
	ContainerID string `json:"containerID"`
	State       struct {
		Waiting *struct {
			Message string `json:"message"`
		} `json:"waiting"`
	} `json:"state"`
}

func parsePodWatchEvent(line []byte) (podWatchEvent, error) {
	var e podWatchEvent
	err := json.Unmarshal(line, &e)
	return e, err
}

// kubectlLogsTailArgs builds the per-container `kubectl logs -f` args.
func kubectlLogsTailArgs(kubeconfig, kctx, ns, pod, container string, sinceSeconds int) []string {
	return kubectlArgs(kubeconfig, kctx,
		"logs", fmt.Sprintf("--since=%ds", sinceSeconds), "-f", pod, "-c", container, "--namespace", ns)
}

// --- runtime watcher ---

type trackedContainers struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (t *trackedContainers) addNew(id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ids[id] {
		return false
	}
	t.ids[id] = true
	return true
}

// startKubernetesLogs watches pods in ns and tails each new container's logs
// into the service sink (inline + file). Returns a cancel func. No-op for LogOff.
func startKubernetesLogs(parent context.Context, n *Node, kubeconfig, kctx, ns string) (context.CancelFunc, error) {
	if n.LogMode == LogOff {
		return func() {}, nil
	}
	w, closeFn, err := logs.BuildSink(n.Name, sinkMode(n.LogMode), os.Stdout, logs.ColorEnabled(), nil)
	if err != nil {
		return nil, err
	}
	n.logCloser = closeFn

	ctx, cancel := context.WithCancel(parent)
	start := time.Now()
	tracked := &trackedContainers{ids: map[string]bool{}}

	go func() {
		// `kubectl get pods -w --output-watch-events -o json` streams one JSON
		// object per line: {"type": "...", "object": {...}}.
		cmd := exec.CommandContext(ctx, "kubectl",
			kubectlArgs(kubeconfig, kctx, "get", "pods", "-n", ns, "-w", "--output-watch-events", "-o", "json")...)
		stdout, err := cmd.StdoutPipe()
		if err != nil || cmd.Start() != nil {
			return
		}
		dec := json.NewDecoder(stdout)
		for {
			var evt podWatchEvent
			if err := dec.Decode(&evt); err != nil {
				return // stream closed or ctx cancelled
			}
			if evt.Type == "DELETED" {
				continue
			}
			all := append(evt.Object.Status.InitContainerStatuses, evt.Object.Status.ContainerStatuses...)
			for _, c := range all {
				if c.ContainerID == "" {
					if c.State.Waiting != nil && c.State.Waiting.Message != "" {
						_, _ = fmt.Fprintf(w, "%s/%s: %s\n", evt.Object.Metadata.Name, c.Name, c.State.Waiting.Message)
					}
					continue
				}
				if tracked.addNew(c.ContainerID) {
					since := int(time.Since(start).Seconds()) + 1
					go tailPodContainer(ctx, w, kubeconfig, kctx, ns, evt.Object.Metadata.Name, c.Name, since)
				}
			}
		}
	}()
	return cancel, nil
}

func tailPodContainer(ctx context.Context, w io.Writer, kubeconfig, kctx, ns, pod, container string, since int) {
	cmd := exec.CommandContext(ctx, "kubectl", kubectlLogsTailArgs(kubeconfig, kctx, ns, pod, container, since)...)
	cmd.Stdout = w
	cmd.Stderr = w
	_ = cmd.Run() // returns when the pod dies or ctx is cancelled
}
