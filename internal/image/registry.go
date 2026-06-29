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

	// minioNamespace / minioRootSecret / minioService describe the in-cluster
	// MinIO the registry's S3 (HA) backend talks to. minioRootSecret holds the
	// rootUser/rootPassword the chart provisions (existingSecret: minio-root).
	minioNamespace  = "minio"
	minioService    = "minio-amd64"
	minioRootSecret = "minio-root"
	// registryBucket is the MinIO bucket holding registry blobs (declared in the
	// minio-amd64 AppSet; also ensured idempotently here for fresh-cluster ordering).
	registryBucket = "registry"
	// registryS3Secret holds the S3 access/secret keys (copied from minioRootSecret)
	// the registry pods consume via REGISTRY_STORAGE_S3_ACCESSKEY/SECRETKEY.
	registryS3Secret = "registry-s3-creds"
)

// registryManifestPVCTmpl deploys the SINGLE-replica registry backed by a
// ReadWriteOnce PVC. Used when MinIO is absent (single-node lima, etc.). Because
// the single replica holds an RWO volume, the Deployment uses Recreate — a
// rolling update would deadlock (new pod can't mount a volume the old pod holds).
// registryStorage picks the class: off-node `nfs` when present (survives a node
// nap), else node-local `local-path`. Images are re-pullable, so even on
// local-path a node nap just means the registry waits for that node.
const registryManifestPVCTmpl = `apiVersion: v1
kind: Namespace
metadata:
  name: devx-registry
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: __PVC_NAME__
  namespace: devx-registry
  labels:
    app: devx-registry
spec:
  storageClassName: __STORAGE_CLASS__
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
            claimName: __PVC_NAME__
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

// registryManifestS3Tmpl deploys the HA registry: 2 replicas (RollingUpdate, soft
// hostname anti-affinity) sharing object storage in MinIO via the registry's S3
// driver, so a node loss never takes the registry down. No PVC — the S3 backend is
// the shared, off-node store. config.yml comes from a ConfigMap; only the S3
// access/secret keys are injected from the registryS3Secret (so no creds in git or
// in the ConfigMap). EnsureRegistry guarantees the bucket + the creds secret before
// this is applied.
const registryManifestS3Tmpl = `apiVersion: v1
kind: Namespace
metadata:
  name: devx-registry
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: registry-config
  namespace: devx-registry
  labels:
    app: devx-registry
data:
  config.yml: |
    version: 0.1
    log:
      level: info
    http:
      addr: :5000
    storage:
      delete:
        enabled: true
      s3:
        regionendpoint: http://minio-amd64.minio.svc.cluster.local:9000
        region: us-east-1
        bucket: registry
        forcepathstyle: true
        secure: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
  namespace: devx-registry
  labels:
    app: devx-registry
spec:
  replicas: 2
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  selector:
    matchLabels:
      app: devx-registry
  template:
    metadata:
      labels:
        app: devx-registry
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                topologyKey: kubernetes.io/hostname
                labelSelector:
                  matchLabels:
                    app: devx-registry
      containers:
        - name: registry
          image: registry:2
          ports:
            - containerPort: 5000
          env:
            # S3 creds injected from the registryS3Secret (copied from minio-root);
            # they override the (credential-free) storage.s3 block in config.yml.
            - name: REGISTRY_STORAGE_S3_ACCESSKEY
              valueFrom:
                secretKeyRef:
                  name: registry-s3-creds
                  key: accesskey
            - name: REGISTRY_STORAGE_S3_SECRETKEY
              valueFrom:
                secretKeyRef:
                  name: registry-s3-creds
                  key: secretkey
          volumeMounts:
            - name: config
              mountPath: /etc/docker/registry
      volumes:
        - name: config
          configMap:
            name: registry-config
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

// registryManifestPVC renders the PVC-backed manifest for the given StorageClass
// and PVC name.
func registryManifestPVC(storageClass, pvcName string) string {
	m := strings.ReplaceAll(registryManifestPVCTmpl, "__STORAGE_CLASS__", storageClass)
	return strings.ReplaceAll(m, "__PVC_NAME__", pvcName)
}

// registryStorage picks the StorageClass + PVC name for the PVC fallback: the
// off-node `nfs` class when present (dev-local, NAS-backed — survives a node
// napping), otherwise the k3s built-in `local-path` (single-node lima, etc.). The
// PVC name differs by class because a PVC's storageClassName is immutable, so a
// class switch needs a distinct PVC (and a one-time data copy).
func registryStorage(ctx context.Context, kubeconfig, kctx string) (storageClass, pvcName string) {
	if _, err := kubectl(ctx, kubeconfig, kctx, "get", "storageclass", "nfs"); err == nil {
		return "nfs", "registry-data-nfs"
	}
	return "local-path", "registry-data"
}

// minioAvailable reports whether the in-cluster MinIO (the registry's S3 HA
// backend) is present on the target. When it is, the registry runs HA (2 replicas
// on S3); otherwise it falls back to the single-replica PVC deployment.
func minioAvailable(ctx context.Context, kubeconfig, kctx string) bool {
	_, err := kubectl(ctx, kubeconfig, kctx, "-n", minioNamespace, "get", "service", minioService)
	return err == nil
}

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
// and reference it. When MinIO is present the registry runs HA (2 replicas on S3);
// otherwise it falls back to a single-replica PVC deployment. kubeconfig and kctx
// target the cluster (already resolved by the caller); kctx may be empty to use the
// current context.
func EnsureRegistry(ctx context.Context, kubeconfig, kctx string) (*LocalRegistry, error) {
	var manifest string
	if minioAvailable(ctx, kubeconfig, kctx) {
		// HA path: ensure the bucket + S3 creds exist before the registry pods
		// (which crash-loop without them), then deploy the 2-replica S3 manifest.
		if err := ensureRegistryBucket(ctx, kubeconfig, kctx); err != nil {
			return nil, err
		}
		if err := ensureRegistryS3Creds(ctx, kubeconfig, kctx); err != nil {
			return nil, err
		}
		manifest = registryManifestS3Tmpl
	} else {
		storageClass, pvcName := registryStorage(ctx, kubeconfig, kctx)
		manifest = registryManifestPVC(storageClass, pvcName)
	}
	if out, err := kubectlApplyStdin(ctx, kubeconfig, kctx, manifest); err != nil {
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

// ensureRegistryBucket idempotently creates the registry's MinIO bucket via mc
// inside a MinIO pod (which already has the root creds in its env). Safe to run
// repeatedly (mc mb --ignore-existing).
func ensureRegistryBucket(ctx context.Context, kubeconfig, kctx string) error {
	pod, err := kubectl(ctx, kubeconfig, kctx, "-n", minioNamespace, "get", "pods",
		"-l", "app=minio", "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return fmt.Errorf("finding a MinIO pod to ensure the registry bucket: %w\n%s", err, strings.TrimSpace(pod))
	}
	pod = strings.TrimSpace(pod)
	if pod == "" {
		return fmt.Errorf("no MinIO pod found to ensure the registry bucket")
	}
	mc := "mc alias set local http://localhost:9000 \"$MINIO_ROOT_USER\" \"$MINIO_ROOT_PASSWORD\" >/dev/null && " +
		"mc mb --ignore-existing local/" + registryBucket
	if out, err := kubectl(ctx, kubeconfig, kctx, "-n", minioNamespace, "exec", pod, "--",
		"sh", "-c", mc); err != nil {
		return fmt.Errorf("ensuring registry bucket %q in MinIO: %w\n%s", registryBucket, err, strings.TrimSpace(out))
	}
	return nil
}

// ensureRegistryS3Creds copies the MinIO root credentials into a Secret in the
// registry namespace (keys accesskey/secretkey) for the registry's S3 driver. The
// values are read base64-encoded from minioRootSecret and written through verbatim
// (a Secret's `data` is base64), so plaintext never appears in a command line.
func ensureRegistryS3Creds(ctx context.Context, kubeconfig, kctx string) error {
	ak, err := kubectl(ctx, kubeconfig, kctx, "-n", minioNamespace, "get", "secret", minioRootSecret,
		"-o", "jsonpath={.data.rootUser}")
	if err != nil {
		return fmt.Errorf("reading MinIO root user for registry S3 creds: %w\n%s", err, strings.TrimSpace(ak))
	}
	sk, err := kubectl(ctx, kubeconfig, kctx, "-n", minioNamespace, "get", "secret", minioRootSecret,
		"-o", "jsonpath={.data.rootPassword}")
	if err != nil {
		return fmt.Errorf("reading MinIO root password for registry S3 creds: %w\n%s", err, strings.TrimSpace(sk))
	}
	ak, sk = strings.TrimSpace(ak), strings.TrimSpace(sk)
	if ak == "" || sk == "" {
		return fmt.Errorf("MinIO root creds (%s/%s) are empty; cannot build registry S3 creds", minioNamespace, minioRootSecret)
	}
	secret := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
  labels:
    app: devx-registry
type: Opaque
data:
  accesskey: %s
  secretkey: %s
`, RegistryNamespace, registryS3Secret, RegistryNamespace, ak, sk)
	if out, err := kubectlApplyStdin(ctx, kubeconfig, kctx, secret); err != nil {
		return fmt.Errorf("applying registry S3 creds secret: %w\n%s", err, strings.TrimSpace(out))
	}
	return nil
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
