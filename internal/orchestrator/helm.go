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
	"os/exec"
	"path/filepath"
	"strings"
)

// validateHelm ensures the helm CLI is available, required for renderer: helm.
func validateHelm() error {
	if _, err := exec.LookPath("helm"); err != nil {
		return fmt.Errorf("kubernetes.renderer \"helm\" requires the helm CLI on your PATH: %w", err)
	}
	return nil
}

// helmArgs builds a fresh helm arg slice with cluster-targeting flags
// (--kubeconfig / --kube-context) and namespace, plus the given subcommand args.
// Returns a NEW slice each call to avoid append-aliasing across invocations.
func helmArgs(kubeconfig, kctx, ns string, extra ...string) []string {
	args := []string{"--kubeconfig", kubeconfig}
	if kctx != "" {
		args = append(args, "--kube-context", kctx)
	}
	if ns != "" {
		args = append(args, "-n", ns)
	}
	return append(args, extra...)
}

func runHelm(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "helm", args...).CombinedOutput()
	return string(out), err
}

// helmUpgradeInstall installs or upgrades release from chart (a chart directory,
// packaged chart, or repo ref), applying any values files. It is idempotent —
// re-running upgrades the existing release.
// helmUpgradeArgs builds the full `helm upgrade --install` argument list. Pure
// function, separated from helmUpgradeInstall for unit-testing the arg shape.
func helmUpgradeArgs(kubeconfig, kctx, ns, release, chart string, values []string) []string {
	extra := []string{"upgrade", "--install", release, chart}
	for _, v := range values {
		extra = append(extra, "-f", v)
	}
	return helmArgs(kubeconfig, kctx, ns, extra...)
}

func helmUpgradeInstall(ctx context.Context, kubeconfig, kctx, ns, release, chart string, values []string) error {
	if out, err := runHelm(ctx, helmUpgradeArgs(kubeconfig, kctx, ns, release, chart, values)...); err != nil {
		return fmt.Errorf("helm upgrade --install %q failed: %w\n%s", release, err, strings.TrimSpace(out))
	}
	return nil
}

// helmUninstall removes a release (best-effort, for cleanup on shutdown).
func helmUninstall(ctx context.Context, kubeconfig, kctx, ns, release string) (string, error) {
	return runHelm(ctx, helmArgs(kubeconfig, kctx, ns, "uninstall", release)...)
}

// resolveValuesPaths resolves (possibly relative) values-file paths against baseDir
// (the service's working directory), mirroring resolveManifestsPath.
func resolveValuesPaths(baseDir string, values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if filepath.IsAbs(v) || v == "" {
			out = append(out, v)
			continue
		}
		out = append(out, filepath.Join(baseDir, v))
	}
	return out
}
