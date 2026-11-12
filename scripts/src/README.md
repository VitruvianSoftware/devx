# Src

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/scripts/src`.

## Interface
- `[lib.rs:ClusterNode]`
- `[lib.rs:MigrateOptions]`
- `[lib.rs:parse_args]`
- `[lib.rs:base64_encode]`
- `[lib.rs:is_local_host]`
- `[lib.rs:detect_local_hostname]`
- `[lib.rs:parse_cluster_yaml]`
- `[lib.rs:compute_rolling_order]`
- `[lib.rs:update_lima_yaml_content]`
- `[lib.rs:patch_k3s_systemd_service]`
- `[lib.rs:build_vm_command]`
- `[lib.rs:run_migration]`

## Invariants
- [lib.rs:424]: `assert!(changed);`
- [lib.rs:425]: `assert!(updated.contains("portForwards:\n  - guestSocket: \"/var/run/docker.s...`
- [lib.rs:429]: `assert!(!changed2);`

## Side Effects
- None

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/scripts/src/...`
