# Task Tracker — Idea #29: Shift-Left Distributed Observability

- `[/]` **1. Core Telemetry Package**
  - `[ ]` Create `internal/telemetry/otel.go` with `SpawnArgs()` and `DiscoverOTEL()`.

- `[ ]` **2. CLI Commands**
  - `[ ]` Create `cmd/trace.go` (root command group)
  - `[ ]` Create `cmd/trace_spawn.go` (jaeger/grafana with `--persist`)
  - `[ ]` Create `cmd/trace_list.go`
  - `[ ]` Create `cmd/trace_rm.go`

- `[ ]` **3. Shell Integration**
  - `[ ]` Modify `cmd/shell.go` to call `telemetry.DiscoverOTEL()` and inject vars

- `[ ]` **4. Verification & Push**
  - `[ ]` `go build && go vet && go test`
  - `[ ]` Execute `/push` workflow
  - `[ ]` Verify CI green on main
