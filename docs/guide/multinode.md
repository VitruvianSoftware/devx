# Multi-Node Clusters

The `devx cluster` command suite allows you to provision and manage a local, multi-node Kubernetes (K3s) cluster using Lima VMs. It's designed for advanced developers whose applications are large enough to require scaling their local Kubernetes development beyond a single laptop or node.

## Configuration

To orchestrate a multi-node cluster, you define a `cluster.yaml` file at the root of your project or workspace. This file describes the desired state of your cluster, including the version of K3s and the specifications for each node.

See `cluster.yaml.example` in the repository root for a complete reference.

```yaml
cluster:
  name: devx-cluster
  k3sVersion: "v1.35.3+k3s1"
  kubeconfig: "~/.kube/cluster.yaml"

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

The cluster manager provides several commands to handle the lifecycle of your multi-node cluster.

### `devx cluster init`

Bootstraps a new cluster from the config file. It will provision the Lima VMs on each configured host, install prerequisites, and bootstrap the initial K3s server in HA mode.

*   **Idempotent**: It skips steps that are already completed.
*   **Dry Run**: Use `-n` or `--dry-run` to see what would happen without making changes.
*   **Auto Install**: Use `--auto-install` to automatically install missing local prerequisites (like `limactl`).

### `devx cluster join`

Joins new or pending agent nodes to the existing cluster. Useful for expanding your cluster after the initial `init`.

### `devx cluster apply`

Reconciles the cluster state. It ensures all running nodes match the specifications in the `cluster.yaml` configuration.

### `devx cluster upgrade`

Performs a rolling upgrade of the K3s version across the cluster according to the `k3sVersion` specified in the configuration. 

### `devx cluster remove`

Gracefully drains and cordons a specific node, and removes it from the Kubernetes cluster.

### `devx cluster destroy`

Tears down the entire cluster. Uninstalls K3s, stops and deletes all Lima VMs, and removes the exported kubeconfig.

*   **Non-Interactive**: Use `-y` or `--non-interactive` to skip the destructive confirmation prompt.
