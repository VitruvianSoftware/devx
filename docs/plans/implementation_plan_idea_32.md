# Zero-Config Local Kubernetes (Idea #32)

This plan details the implementation of `devx k8s spawn`, which provisions a local Kubernetes cluster natively within `devx` by orchestrating the container directly.

## K3s vs. Kind Architecture Note
> You asked: *"Isn't Kind the closest to vanilla k8s and lightweight also?"*

You are absolutely correct that `Kind` is the closest to vanilla Kubernetes. However, the `Kind` CLI achieves this by pulling a full OS-in-a-container image (`kindest/node`) and then executing complex multi-step `kubeadm init` and `kubelet` bootstrapping commands *inside* that container over time. Replicating the logic of the `kind` CLI entirely from scratch natively in Go without forcing the developer to `brew install kind` is incredibly complex. 

`K3s`, on the other hand, packages the entire Kubernetes control plane into a **single binary**. Running `podman run rancher/k3s server` instantly boots a fully functional cluster in one step without any complex multi-stage orchestration. This single-binary architecture makes `k3s` the superior choice for embedding gracefully inside `devx` to achieve true "Zero-Install / Zero-Config" local Kubernetes!

## Proposed Changes

### 1. Isolated Config Generation (Option B)
To ensure absolute safety for the developer's personal host configuration:
1. When a k3s cluster is spawned, we will extract its `/etc/rancher/k3s/k3s.yaml`.
2. We will rewrite the server IP to point to localhost with the dynamically mapped port.
3. We will save it strictly to an isolated file: `~/.kube/devx-<name>.yaml`.
4. We will instruct the user to use `export KUBECONFIG=~/.kube/devx-<name>.yaml`, and automatically inject this into the `devx shell` environment when active.

### 2. New Commands (`cmd/k8s*.go`)
Create the standard lifecycle tree:
- `devx k8s`: Root namespace
- `devx k8s spawn [name]`: Spawns a new k3s single-node cluster (default name: `local`).
- `devx k8s list`: Lists running k3s clusters and their connection `KUBECONFIG` paths.
- `devx k8s rm [name]`: Tears down the container and deletes the isolated `~/.kube/devx-<name>.yaml` file.

### 3. K3s Engine (`internal/k8s/k3s.go`)
- **Container Image:** `docker.io/rancher/k3s:v1.30.0-k3s1` (Tracking a stable v1.30 release).
- **Execution:** Runs via `podman run -d --privileged --name devx-k8s-<name> -p <port>:6443 ...`
- **Tolerations:** Uses `--tls-san 127.0.0.1` flag on the `k3s server` command to ensure the TLS cert covers the port-forwarded local API calls without x509 SAN errors.

## Verification & Documentation Plan

1. I will execute `devx k8s spawn local` entirely via our automated tools to boot a cluster.
2. I will utilize `devx k8s list` to observe the generated `KUBECONFIG` path.
3. I will utilize standard `kubectl` pointing at the new isolated context to deploy a simple test pod and run `kubectl get nodes,pods`.
4. Using the browser subagent, I will render this sequential terminal execution into an aesthetic UI layout and capture a high-quality screenshot.
5. I will embed this screenshot directly into a new `/guide/kubernetes` VitePress documentation page as the official visual proof.
