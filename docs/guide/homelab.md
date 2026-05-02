# Homelab Manager

The `devx homelab` command suite allows you to provision and manage a local, multi-node Kubernetes (K3s) cluster using Lima VMs. It's designed to simulate a real-world edge or distributed "homelab" environment directly on your local development machine(s).

## Configuration

To orchestrate a homelab, you define a `homelab.yaml` file at the root of your project or workspace. This file describes the desired state of your cluster, including the version of K3s and the specifications for each node.

See `homelab.yaml.example` in the repository root for a complete reference.

```yaml
cluster:
  name: devx-homelab
  k3sVersion: "v1.35.3+k3s1"
  kubeconfig: "~/.kube/homelab.yaml"

nodes:
  - host: james-mbp
    role: server
    pool: laptop-cp-1
    vm:
      cpus: 4
      memory: 8GiB
      disk: 30GiB
```

## Commands

The homelab manager provides several commands to handle the lifecycle of your cluster.

### `devx homelab init`

Bootstraps a new cluster from the config file. It will provision the Lima VMs on each configured host, install prerequisites, and bootstrap the initial K3s server in HA mode.

*   **Idempotent**: It skips steps that are already completed.
*   **Dry Run**: Use `-n` or `--dry-run` to see what would happen without making changes.
*   **Auto Install**: Use `--auto-install` to automatically install missing local prerequisites (like `limactl`).

### `devx homelab join`

Joins new or pending agent nodes to the existing homelab cluster. Useful for expanding your cluster after the initial `init`.

### `devx homelab apply`

Reconciles the cluster state. It ensures all running nodes match the specifications in the `homelab.yaml` configuration.

### `devx homelab upgrade`

Performs a rolling upgrade of the K3s version across the cluster according to the `k3sVersion` specified in the configuration. 

### `devx homelab remove`

Gracefully drains and cordons a specific node, and removes it from the Kubernetes cluster.

### `devx homelab destroy`

Tears down the entire cluster. Uninstalls K3s, stops and deletes all Lima VMs, and removes the exported kubeconfig.

*   **Non-Interactive**: Use `-y` or `--non-interactive` to skip the destructive confirmation prompt.
