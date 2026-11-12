# State

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/state`.

## Interface
- `[checkpoint.go:CheckpointsDir]`
- `[checkpoint.go:CreateCheckpoint]`
- `[checkpoint.go:RestoreCheckpoint]`
- `[checkpoint.go:CheckpointInfo]`
- `[checkpoint.go:ListCheckpoints]`
- `[checkpoint.go:DeleteCheckpoint]`
- `[crypto.go:GeneratePassphrase]`
- `[crypto.go:EncryptFile]`
- `[crypto.go:DecryptFile]`
- `[dump.go:DumpReport]`
- `[dump.go:VMStatus]`
- `[dump.go:ContainerDiagnostic]`
- `[dump.go:TopologyStatus]`
- `[dump.go:TunnelDiagnostic]`
- `[dump.go:RedactedConfig]`
- `[dump.go:GenerateDump]`
- `[dump.go:GenerateMarkdown]`
- `[dump.go:GenerateJSON]`
- `[relay.go:ParseRelay]`
- `[relay.go:UploadToS3]`

## Invariants
- None

## Side Effects
- [checkpoint.go:26]: `"os/exec"`
- [checkpoint.go:94]: `cmd := exec.Command("podman", "container", "checkpoint", containerId, "-e", a...`
- [checkpoint.go:145]: `cmd := exec.Command("podman", "container", "restore", "-i", arch)`
- [crypto.go:157]: `if err := os.WriteFile(outputPath, plaintext, 0o644); err != nil {`
- [dump.go:104]: `out, err := prov.Runtime.Exec("ps", "-a", "--filter", "name=devx-", "--format...`
- [dump.go:120]: `logOut, _ := prov.Runtime.Exec("logs", "--tail", "25", cd.Name)`
- [relay.go:25]: `"os/exec"`
- [relay.go:31]: `// TODO(Idea 56): Implement HTTP relay support (e.g., https://transfer.sh) as...`
- [relay.go:46]: `// TODO(Idea 56): Support HTTP URLs here (backend="http")`
- [replication.go:103]: `srcTar := filepath.Join(database.SnapshotDir(), db.Engine, db.Name+".tar")`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/state/...`
