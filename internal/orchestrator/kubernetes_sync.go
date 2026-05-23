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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// KubeSync maps a local source directory to a path inside a deployed pod for
// live-reload: after deploy, devx copies src into the pod and re-copies it on
// change, so an in-pod watch process (e.g. `tsx watch`, nodemon) hot-reloads.
type KubeSync struct {
	Src       string // local source dir (resolved against the service's working dir)
	Dest      string // destination path inside the pod
	Selector  string // pod label selector, e.g. "app=brms-offer"
	Container string // container in the pod (optional; default: the pod's first container)
}

// podSyncInterval is how often the source tree is polled for changes.
const podSyncInterval = 1 * time.Second

// syncIgnoreDirs are directories never synced or scanned (vendored deps, VCS, build output).
var syncIgnoreDirs = map[string]bool{
	".git": true, "node_modules": true, ".devx": true, "__pycache__": true,
	".next": true, ".nuxt": true, "dist": true, "build": true,
}

// startPodSync starts a watcher per sync mapping that copies local changes into the
// matching pod, and returns a cancel func that stops them all (called by the DAG on
// shutdown). Initial-setup errors are returned; per-change failures are logged so a
// transient hiccup doesn't tear down the dev loop.
func startPodSync(parent context.Context, n *Node, kubeconfig, kctx, ns string, syncs []KubeSync) (context.CancelFunc, error) {
	if len(syncs) == 0 {
		return func() {}, nil
	}
	base := n.Dir
	if base == "" {
		base, _ = os.Getwd()
	}
	ctx, cancel := context.WithCancel(parent)
	for _, s := range syncs {
		if s.Src == "" || s.Dest == "" || s.Selector == "" {
			cancel()
			return nil, fmt.Errorf("sync: src, dest, and selector are all required")
		}
		src := s.Src
		if !filepath.IsAbs(src) {
			src = filepath.Join(base, src)
		}
		pod, err := discoverPod(ctx, kubeconfig, kctx, ns, s.Selector)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("sync: %w", err)
		}
		if err := kubectlCp(ctx, kubeconfig, kctx, ns, pod, s.Container, src, s.Dest); err != nil {
			cancel()
			return nil, fmt.Errorf("sync: initial copy %s → %s/%s:%s: %w", src, ns, pod, s.Dest, err)
		}
		fmt.Printf("  🔁 live-reload: %s → pod %s:%s\n", src, pod, s.Dest)
		go watchAndSync(ctx, kubeconfig, kctx, ns, s, src)
	}
	return cancel, nil
}

// watchAndSync polls src and re-copies it into the matching pod whenever the source
// signature changes, until ctx is cancelled.
func watchAndSync(ctx context.Context, kubeconfig, kctx, ns string, s KubeSync, src string) {
	last, _ := dirSignature(src)
	ticker := time.NewTicker(podSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sig, err := dirSignature(src)
			if err != nil || sig == last {
				continue
			}
			last = sig
			pod, err := discoverPod(ctx, kubeconfig, kctx, ns, s.Selector)
			if err != nil {
				continue // pod not ready (e.g. rolling); retry next tick
			}
			if err := kubectlCp(ctx, kubeconfig, kctx, ns, pod, s.Container, src, s.Dest); err != nil {
				if ctx.Err() == nil {
					fmt.Printf("  ⚠️  live-reload: sync to %s failed: %v\n", pod, err)
				}
			}
		}
	}
}

// discoverPod returns the first Running pod name matching selector in ns.
func discoverPod(ctx context.Context, kubeconfig, kctx, ns, selector string) (string, error) {
	out, err := runKubectl(ctx, kubectlArgs(kubeconfig, kctx, "get", "pods", "-n", ns,
		"-l", selector, "--field-selector=status.phase=Running",
		"-o", "jsonpath={.items[0].metadata.name}")...)
	if err != nil {
		return "", fmt.Errorf("discovering pod (selector %q): %w\n%s", selector, err, strings.TrimSpace(out))
	}
	pod := strings.TrimSpace(out)
	if pod == "" {
		return "", fmt.Errorf("no running pod matched selector %q in namespace %q", selector, ns)
	}
	return pod, nil
}

// kubectlCp copies a local path into a pod via `kubectl cp`.
func kubectlCp(ctx context.Context, kubeconfig, kctx, ns, pod, container, src, dest string) error {
	cpArgs := []string{"cp", src, fmt.Sprintf("%s/%s:%s", ns, pod, dest)}
	if container != "" {
		cpArgs = append(cpArgs, "-c", container)
	}
	if out, err := runKubectl(ctx, kubectlArgs(kubeconfig, kctx, cpArgs...)...); err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(out))
	}
	return nil
}

// dirSignature computes a cheap change-detection signature of src (file count,
// total size, and newest mtime), skipping syncIgnoreDirs. A change in any of the
// three triggers a re-sync.
func dirSignature(src string) (string, error) {
	var count, size, maxMod int64
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if syncIgnoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // a file vanished mid-walk; ignore it
		}
		count++
		size += info.Size()
		if m := info.ModTime().UnixNano(); m > maxMod {
			maxMod = m
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%d-%d", count, size, maxMod), nil
}
