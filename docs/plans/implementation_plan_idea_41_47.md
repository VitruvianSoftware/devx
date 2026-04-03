# Shareable Diagnostic Dumps (Idea 41) & Time-Travel Checkpoints (Idea 47)

This feature introduces the `devx state` command hierarchy, which will manage the entire topological state of the local environment. It serves two distinct purposes:
1. **Diagnostic Dumps (`devx state dump`)**: Generates a structured JSON or Markdown snapshot of the environment's health, topology, tools, credentials, and crash logs for remote debugging context sharing.
2. **Time-Travel Debugging (`devx state checkpoint` / `devx state restore`)**: Uses Podman's CRIU capabilities to snapshot the entire topology's RAM, active network sockets, and volumes, allowing developers to instantly "rewind" their architecture back to a known-good state.

## Proposed Changes

### `cmd/state.go`
Introduce a top-level `state` command grouping to house `devx state dump`, `devx state checkpoint`, and `devx state restore`.
#### [NEW] [state.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state.go)

### `cmd/state_dump.go`
Implementation of the `devx state dump` subcommand.
- By default, it will output a cleanly formatted Markdown report to `stdout`.
- Included `--file <path>` flag to write directly to disk.
- With `--json`, it will output a strictly structured JSON envelope for programmatic ingestion.
- **Redaction Policy:** `devx.yaml` container images and topologies remain fully visible, but any raw string literals hardcoded inside `env:` blocks will be replaced with `<REDACTED>` to ensure the snapshot is safe to share publicly.
#### [NEW] [state_dump.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_dump.go)

### `cmd/state_checkpoint.go` & `cmd/state_restore.go`
Implementation of the full-topology checkpointing system.
- `devx state checkpoint <name>`: Discovers all currently running devx-managed containers. Calls `podman container checkpoint <id> --export ~/.devx/checkpoints/<name>/<id>.tar.gz` (and optionally pauses them). It will also capture any associated volumes.
- `devx state restore <name>`: Tears down the current running state and runs `podman container restore --import` for the requested checkpoint.
- Includes `devx state list` and `devx state rm` to manage checkpoints.
#### [NEW] [state_checkpoint.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_checkpoint.go)
#### [NEW] [state_restore.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_restore.go)

### `internal/state/dump.go` & `internal/state/checkpoint.go`
Create the `internal/state` package that manages the orchestration.
- **Dump Engine:** Uses `doctor.CheckSystem()` and `doctor.RunFullAudit()`. Queries VM, Cloudflare status, running DBs/services, and tails the last 50 lines of any crashing containers.
- **Checkpoint Engine:** Wraps Podman's CRIU commands. Since `podman checkpoint` is experimental on macOS VMs, we will add safe degradation (e.g., stopping/backing up volumes if CRIU fails, or ensuring the containers are stateless enough for a restart). CRIU requires the `--keep` flag for volumes. Note: Docker Desktop does not reliably support CRIU without experimental flags enabled on the daemon mode, so checkpoints may only be fully supported when `--provider=podman` is active.
#### [NEW] [dump.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/dump.go)
#### [NEW] [checkpoint.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/checkpoint.go)

## Verification Plan

### Automated Verification
- Run `go build ./...` and `go test ./...`
- Verify that secret keys are strictly obfuscated in the JSON/Markdown dump exports.

### Manual Verification (Dogfooding)
- Run `devx state dump` with both broken and working environments to verify log gathering and redaction.
- Create a `devx state checkpoint pristine`, mutate the database or file state inside a container, then `devx state restore pristine` to verify the time-travel rollback capability.
