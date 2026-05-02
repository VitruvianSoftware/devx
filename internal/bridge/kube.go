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

// Package bridge implements the hybrid edge-to-local cloud routing primitives
// for Idea 46 (devx bridge). It orchestrates kubectl port-forward subprocesses,
// manages session state, and generates bridge environment variables for devx shell.
//
// Design: All cluster interactions use the kubectl subprocess pattern, consistent
// with devx's established pattern of wrapping external CLIs (cloudflared, podman, etc.).
//
// Future: dns.go is reserved for Idea 46.1.5 (DNS Proxy) — a lightweight DNS
// proxy using Go's net package that resolves *.svc.cluster.local to forwarded
// local ports without requiring elevated permissions.
package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/devxerr"
)

// ValidateKubectl checks that kubectl is on the PATH and returns its version.
func ValidateKubectl() (string, error) {
	path, err := exec.LookPath("kubectl")
	if err != nil {
		return "", devxerr.New(devxerr.CodeBridgeKubeconfigNotFound,
			"kubectl not found on PATH — install it: https://kubernetes.io/docs/tasks/tools/", nil)
	}

	out, err := exec.Command(path, "version", "--client", "--output=json").Output()
	if err != nil {
		return path, nil // kubectl exists but version check failed — not fatal
	}

	var versionInfo struct {
		ClientVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"clientVersion"`
	}
	if json.Unmarshal(out, &versionInfo) == nil && versionInfo.ClientVersion.GitVersion != "" {
		return versionInfo.ClientVersion.GitVersion, nil
	}
	return path, nil
}

// ResolveKubeconfig returns the absolute path to the kubeconfig file.
// It checks, in order: explicit path > KUBECONFIG env var > ~/.kube/config.
func ResolveKubeconfig(explicit string) (string, error) {
	candidates := []string{explicit}

	if envKC := os.Getenv("KUBECONFIG"); envKC != "" {
		candidates = append(candidates, envKC)
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".kube", "config"))
	}

	for _, path := range candidates {
		if path == "" {
			continue
		}
		// Expand ~ prefix
		if strings.HasPrefix(path, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				path = filepath.Join(home, path[2:])
			}
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}

	return "", devxerr.New(devxerr.CodeBridgeKubeconfigNotFound,
		"kubeconfig not found — specify with --kubeconfig or set KUBECONFIG", nil)
}

// ValidateContext checks that the specified context exists and the cluster is reachable.
func ValidateContext(kubeconfig, context string) error {
	args := []string{"cluster-info", "--kubeconfig", kubeconfig}
	if context != "" {
		args = append(args, "--context", context)
	}

	out, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "was refused") || strings.Contains(outStr, "Unable to connect") ||
			strings.Contains(outStr, "dial tcp") || strings.Contains(outStr, "context deadline exceeded") {
			return devxerr.New(devxerr.CodeBridgeContextUnreachable,
				fmt.Sprintf("cluster unreachable — are you connected to VPN?\n\n  kubectl output: %s", strings.TrimSpace(outStr)), err)
		}
		if strings.Contains(outStr, "context") && strings.Contains(outStr, "does not exist") {
			return devxerr.New(devxerr.CodeBridgeContextUnreachable,
				fmt.Sprintf("context %q not found in kubeconfig %s", context, kubeconfig), err)
		}
		return devxerr.New(devxerr.CodeBridgeContextUnreachable,
			fmt.Sprintf("kubectl cluster-info failed: %s", strings.TrimSpace(outStr)), err)
	}
	return nil
}

// ValidateService checks that a service exists in the given namespace.
func ValidateService(kubeconfig, context, namespace, service string) error {
	args := []string{
		"get", "svc", service,
		"--kubeconfig", kubeconfig,
		"-n", namespace,
		"-o", "name",
	}
	if context != "" {
		args = append(args, "--context", context)
	}

	out, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "not found") || strings.Contains(outStr, "NotFound") {
			return devxerr.New(devxerr.CodeBridgeServiceNotFound,
				fmt.Sprintf("service %q not found in namespace %q", service, namespace), err)
		}
		if strings.Contains(outStr, "namespaces") && strings.Contains(outStr, "not found") {
			return devxerr.New(devxerr.CodeBridgeNamespaceNotFound,
				fmt.Sprintf("namespace %q does not exist in this cluster", namespace), err)
		}
		return devxerr.New(devxerr.CodeBridgeServiceNotFound,
			fmt.Sprintf("cannot verify service %s/%s: %s", namespace, service, strings.TrimSpace(outStr)), err)
	}
	_ = out
	return nil
}

// ListServices returns the names of services in the given namespace for TUI selection.
func ListServices(kubeconfig, context, namespace string) ([]string, error) {
	args := []string{
		"get", "svc",
		"--kubeconfig", kubeconfig,
		"-n", namespace,
		"-o", "jsonpath={.items[*].metadata.name}",
	}
	if context != "" {
		args = append(args, "--context", context)
	}

	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("listing services in %s: %w", namespace, err)
	}

	names := strings.Fields(strings.TrimSpace(string(out)))
	return names, nil
}

// ListContexts returns available kube contexts from the kubeconfig.
func ListContexts(kubeconfig string) ([]string, error) {
	args := []string{
		"config", "get-contexts",
		"--kubeconfig", kubeconfig,
		"-o", "name",
	}

	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("listing kube contexts: %w", err)
	}

	var contexts []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			contexts = append(contexts, line)
		}
	}
	return contexts, nil
}
