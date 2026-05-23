# Kubernetes Deploy

Deploy a service straight to a Kubernetes cluster as part of `devx up`. devx renders your manifests, applies them to the target cluster, and gates the rest of the DAG on the workload becoming ready. This is the skaffold-class **deploy** path: declare `runtime: kubernetes` on a service and `devx` owns the render → apply → readiness → teardown lifecycle.

::: tip Deploy vs. Spawn vs. Bridge
- **[Zero-Config Kubernetes](kubernetes.md)** (`devx k8s spawn`) gives you a *cluster*.
- **Kubernetes Deploy** (`runtime: kubernetes`, this page) deploys your *app* onto a cluster.
- **[Hybrid Bridge](bridge.md)** (`runtime: bridge`) connects your *local* process to a remote cluster.

They compose: spawn a local cluster, deploy your app onto it, and bridge a remote dependency — all from one `devx.yaml`.
:::

## Overview

A `runtime: kubernetes` service points at a directory of manifests and a renderer. On `devx up`, devx:

1. **Validates** `kubectl` and your cluster access (kubeconfig + context).
2. **Ensures** the target namespace exists (created idempotently if missing).
3. **Applies** the rendered manifests with `kubectl apply` — `-k` for kustomize, `-f` for raw.
4. **Readiness-gates** the DAG: blocks until the namespace's Deployments report `Available` (`kubectl wait --for=condition=Available deployment --all`). The generic HTTP/TCP healthcheck is skipped — in-cluster readiness needs no port-forward.
5. **Prunes** what it applied on shutdown (`Ctrl+C` / cleanup).

Because devx shells out to `kubectl`, kustomize is built in — no extra tooling to install.

## Prerequisites

- `kubectl` installed and on your PATH
- A reachable cluster and a kubeconfig/context with apply permissions

Any cluster works: a [`devx k8s spawn`](kubernetes.md) local cluster, a lima/k3s VM, kind/k3d, or a remote dev cluster. Point at it with `context` / `kubeconfig` (defaults to your current context).

::: warning Target the right cluster
`runtime: kubernetes` applies real manifests to whatever context it resolves. Pin `context:` explicitly when your kubeconfig holds production contexts, so `devx up` can never deploy to the wrong cluster.
:::

## Quick Start

### Configuration in `devx.yaml`

```yaml
services:
  - name: payments-api
    runtime: kubernetes
    depends_on:
      - name: postgres
        condition: service_healthy
    kubernetes:
      manifests: ./deploy/k8s/overlays/local   # kustomize dir (default), raw file/dir, or helm chart
      renderer: kustomize                       # kustomize (default) | raw | helm
      namespace: payments-dev                   # default: "default" (created if missing)
      context: ""                               # default: current kube context
      kubeconfig: ""                            # default: $KUBECONFIG, then ~/.kube/config
```

### Run

```bash
devx up
```

```text
📋 Starting tier 1: payments-api
  ☸️  Deploying payments-api (kustomize) ./deploy/k8s/overlays/local → ns/payments-dev
  ⏳ Waiting for payments-api deployments to become Available...
  ✅ payments-api deployed
  ✅ payments-api is healthy (deployment available)

✅ All services are running and healthy.
```

## Renderers

| Renderer | kubectl flag | Notes |
|----------|--------------|-------|
| `kustomize` (default) | `apply -k` | `manifests` is a directory containing `kustomization.yaml`. Uses kubectl's built-in kustomize — no separate binary. |
| `raw` | `apply -f` | `manifests` is a plain YAML file, or a directory of manifests. |
| `helm` | — (uses `helm`) | Deploys via `helm upgrade --install`; `manifests` is a **chart directory**. Supports `release` + `values` (see below). Requires the `helm` CLI on your PATH. |

### Helm

For `renderer: helm`, `manifests` points at a **chart directory**. devx runs `helm upgrade --install` (idempotent — re-running upgrades the release) and tears it down with `helm uninstall` on shutdown. The release name defaults to the service name (override with `release`); `values` lists values files (resolved against the service directory):

```yaml
services:
  - name: payments-api
    runtime: kubernetes
    kubernetes:
      renderer: helm
      manifests: ./charts/payments-api   # chart directory
      namespace: payments-dev
      release: payments-api              # optional (default: the service name)
      values:                            # optional values files
        - values-local.yaml
```

Requires the `helm` CLI on your PATH. The readiness gate still applies — devx waits for the release's Deployments to become `Available`.

## Cluster & Namespace Targeting

| Field | Default | Notes |
|-------|---------|-------|
| `kubeconfig` | `$KUBECONFIG`, then `~/.kube/config` | An explicit path wins. |
| `context` | current context | Pin this when your kubeconfig holds multiple (or production) clusters. |
| `namespace` | `default` | Created idempotently if it doesn't exist. |

Relative `manifests` paths resolve against the service's directory, so [multirepo includes](multirepo.md) work correctly.

## Profiles

`runtime: kubernetes` participates in [Environment Profiles](orchestration.md#environment-profiles-profile). A common pattern is a base config with a lightweight local runtime and a `k8s` profile that flips a service onto a cluster:

```yaml
services:
  - name: payments-api
    runtime: host
    command: ["go", "run", "./cmd/api"]
    port: 8080

profiles:
  k8s:
    services:
      - name: payments-api
        runtime: kubernetes
        kubernetes:
          manifests: ./deploy/k8s/overlays/local
          namespace: payments-dev
```

```bash
devx up --profile k8s
```

The profile's `kubernetes:` block is merged onto the base service (profile wins).

## Building & Loading Images

devx can build your service images and load them into the cluster before applying, so `runtime: kubernetes` works with locally-built code — not just pre-published images. devx stands up a small **in-cluster registry** (a `registry:2` Deployment on a fixed NodePort, in namespace `devx-registry`), builds each image with your configured container provider, and pushes it there. Reference the image in your manifests as `localhost:30500/<name>:<tag>` (with `imagePullPolicy: Always`) — every node pulls it from its own NodePort, so no per-node registry configuration is required.

```yaml
services:
  - name: payments-api
    runtime: kubernetes
    kubernetes:
      manifests: ./deploy/k8s/overlays/local
      images:
        - name: payments-api      # → pushed as localhost:30500/payments-api:dev
          context: ./payments-api  # build context (default ".")
          dockerfile: Dockerfile   # relative to context (default "Dockerfile")
          tag: dev                 # default "dev"
          # platforms: [linux/amd64, linux/arm64]   # multi-arch (default: the builder's arch)
```

Reference the built image in your manifests:

```yaml
spec:
  containers:
    - name: payments-api
      image: localhost:30500/payments-api:dev
      imagePullPolicy: Always
```

On `devx up`, devx builds and pushes before applying:

```text
  🗄️  in-cluster registry ready (push 100.x.y.z:30500 · ref localhost:30500)
  🔨 building payments-api (lima)...
  📦 loaded payments-api — reference it in manifests as localhost:30500/payments-api:dev
```

::: tip Mixed-architecture clusters
If your cluster mixes amd64 and arm64 nodes, a single-arch image fails to pull on the other architecture (`no matching manifest for ...`). Build multi-arch with `platforms: [linux/amd64, linux/arm64]`, or pin pods to a matching arch via a `kubernetes.io/arch` nodeSelector.
:::

Building requires a working container engine for your devx provider (podman, docker, or a Lima/Colima VM with nerdctl).

## Live-reload (file sync)

`kubernetes.sync` keeps local source in sync with the running pod for hot-reload: after deploy, devx copies each `src` into the matching pod and re-copies it on change (polled ~1s), so an in-pod watch process (`tsx watch`, `nodemon`, `air`, …) reloads. The watchers stop on shutdown.

```yaml
services:
  - name: payments-api
    runtime: kubernetes
    kubernetes:
      manifests: ./deploy/k8s/overlays/local
      sync:
        - src: ./src                    # local dir (relative to the service dir)
          dest: /app/src                # path inside the pod
          selector: app=payments-api    # pod label selector
          # container: payments-api     # optional (default: the pod's first container)
```

The pod's image needs `tar` (used by `kubectl cp`); `node_modules`, `.git`, `dist`, `build`, etc. are skipped; and the in-pod process is responsible for reloading on the synced changes (run it in watch mode).

## Lifecycle & Cleanup

The deploy is a first-class DAG node: it respects `depends_on` ordering, gates dependents on readiness, and is torn down in reverse order on shutdown. devx records exactly what it applied (resolved manifests path, renderer, namespace, context) and runs `kubectl delete … --ignore-not-found` on exit. The namespace itself is left in place.

## Limitations & Roadmap

- **Readiness gate** waits on `Deployment`s in the target namespace. Workloads that ship only `StatefulSet`/`Job`/`CronJob`, or manifests that hardcode a different namespace, aren't covered by the gate yet.
- **Live-reload** uses ~1s polling + `kubectl cp` (whole-directory) — fsnotify-based instant sync and per-file copies are a future optimization.
