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
| `helm` | — | Recognized but **not yet implemented** — fails fast with a clear error. Use `kustomize` or `raw`. (Roadmap.) |

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

## Lifecycle & Cleanup

The deploy is a first-class DAG node: it respects `depends_on` ordering, gates dependents on readiness, and is torn down in reverse order on shutdown. devx records exactly what it applied (resolved manifests path, renderer, namespace, context) and runs `kubectl delete … --ignore-not-found` on exit. The namespace itself is left in place.

## Limitations & Roadmap

- **Readiness gate** waits on `Deployment`s in the target namespace. Workloads that ship only `StatefulSet`/`Job`/`CronJob`, or manifests that hardcode a different namespace, aren't covered by the gate yet.
- **`helm` renderer** is recognized but not implemented.
- **Image build/load** (`kubernetes.images`) is parsed but not yet wired — building service images and loading them into the local cluster is on the roadmap.
- **Live-reload (sync)** into running pods is planned.
