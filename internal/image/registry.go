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

package image

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const (
	// RegistryNamespace is where devx's in-cluster registry lives.
	RegistryNamespace = "devx-registry"
	// RegistryNodePort is the fixed NodePort the registry is exposed on, so
	// manifests can deterministically reference localhost:<RegistryNodePort>/<img>.
	RegistryNodePort = 30500
	registryName     = "registry"
)

// registryManifest deploys the in-cluster registry: a registry:2 Deployment plus
// a NodePort Service. Nodes pull via localhost:<NodePort> (Docker's 127.0.0.0/8
// insecure default covers it — validated on the target cluster, no per-node
// config).
//
// Storage is a PersistentVolumeClaim (not emptyDir): pushed images survive a
// registry pod restart, and — on the cluster default StorageClass (Longhorn on
// dev-local) — survive node loss too, relocating with the volume. The PVC omits
// storageClassName so it binds to whatever default the cluster has (Longhorn here;
// k3s local-path elsewhere), never blocking on a class that may not exist yet.
// Because the single replica holds an RWO volume, the Deployment uses the Recreate
// strategy — a rolling update would otherwise deadlock (the new pod can't mount a
// volume the old pod still holds).
const registryManifest = `apiVersion: v1
kind: Namespace
metadata:
  name: devx-registry
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: registry-data
  namespace: devx-registry
  labels:
    app: devx-registry
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
  namespace: devx-registry
  labels:
    app: devx-registry
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: devx-registry
  template:
    metadata:
      labels:
        app: devx-registry
    spec:
      containers:
        - name: registry
          image: registry:2
          ports:
            - containerPort: 5000
          volumeMounts:
            - name: data
              mountPath: /var/lib/registry
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: registry-data
---
apiVersion: v1
kind: Service
metadata:
  name: registry
  namespace: devx-registry
spec:
  type: NodePort
  selector:
    app: devx-registry
  ports:
    - port: 5000
      targetPort: 5000
      nodePort: 30500
`

// LocalRegistry describes a deployed in-cluster registry and the addresses needed
// to use it.
type LocalRegistry struct {
	// PushAddr is the host→registry address for pushing images, e.g.
	// "100.98.214.120:30500" (a node's reachable address + the NodePort).
	PushAddr string
	// RefPrefix is the in-cluster image-reference prefix, e.g. "localhost:30500".
	// Built images are pushed as <PushAddr>/<name>:<tag> and referenced in
	// manifests as <RefPrefix>/<name>:<tag>.
	RefPrefix string
}

// Ref returns the in-cluster image reference for name:tag (what manifests use).
func (r *LocalRegistry) Ref(name, tag string) string {
	return fmt.Sprintf("%s/%s:%s", r.RefPrefix, name, tag)
}

// PushRef returns the host-side push reference for name:tag.
func (r *LocalRegistry) PushRef(name, tag string) string {
	return fmt.Sprintf("%s/%s:%s", r.PushAddr, name, tag)
}

// EnsureRegistry idempotently deploys the in-cluster registry to the target
// cluster, waits for it to become Available, and returns the addresses to push to
// and reference it. kubeconfig and kctx target the cluster (already resolved by
// the caller); kctx may be empty to use the current context.
func EnsureRegistry(ctx context.Context, kubeconfig, kctx string) (*LocalRegistry, error) {
	if out, err := kubectlApplyStdin(ctx, kubeconfig, kctx, registryManifest); err != nil {
		return nil, fmt.Errorf("deploying in-cluster registry: %w\n%s", err, strings.TrimSpace(out))
	}
	if out, err := kubectl(ctx, kubeconfig, kctx, "-n", RegistryNamespace, "wait",
		"--for=condition=Available", "deployment/"+registryName, "--timeout=120s"); err != nil {
		return nil, fmt.Errorf("waiting for in-cluster registry to become Available: %w\n%s", err, strings.TrimSpace(out))
	}
	nodeAddr, err := firstNodeAddress(ctx, kubeconfig, kctx)
	if err != nil {
		return nil, err
	}
	return &LocalRegistry{
		PushAddr:  fmt.Sprintf("%s:%d", nodeAddr, RegistryNodePort),
		RefPrefix: fmt.Sprintf("localhost:%d", RegistryNodePort),
	}, nil
}

// firstNodeAddress resolves a node InternalIP reachable from the host for pushing
// to the registry's NodePort.
func firstNodeAddress(ctx context.Context, kubeconfig, kctx string) (string, error) {
	out, err := kubectl(ctx, kubeconfig, kctx, "get", "nodes",
		"-o", `jsonpath={.items[0].status.addresses[?(@.type=="InternalIP")].address}`)
	if err != nil {
		return "", fmt.Errorf("resolving a node address for registry push: %w\n%s", err, strings.TrimSpace(out))
	}
	addr := strings.TrimSpace(out)
	if addr == "" {
		return "", fmt.Errorf("could not resolve any node InternalIP for registry push")
	}
	return addr, nil
}

// kubectl runs a kubectl command against the target cluster and returns combined output.
func kubectl(ctx context.Context, kubeconfig, kctx string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "kubectl", kubectlArgs(kubeconfig, kctx, args...)...).CombinedOutput()
	return string(out), err
}

// kubectlApplyStdin runs `kubectl apply -f -`, piping the manifest on stdin.
func kubectlApplyStdin(ctx context.Context, kubeconfig, kctx, manifest string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", kubectlArgs(kubeconfig, kctx, "apply", "-f", "-")...)
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// kubectlArgs builds a fresh kubectl arg slice with --kubeconfig/--context flags
// plus the given subcommand args (fresh slice per call to avoid append-aliasing).
func kubectlArgs(kubeconfig, kctx string, extra ...string) []string {
	args := []string{"--kubeconfig", kubeconfig}
	if kctx != "" {
		args = append(args, "--context", kctx)
	}
	return append(args, extra...)
}
