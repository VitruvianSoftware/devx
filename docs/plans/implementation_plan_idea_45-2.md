# Idea 45.2: Declarative Pipeline Stages (Skaffold-Inspired)

## Background

Today, `devx agent ship` hard-codes per-stack commands in `DetectStack()`. This works for shipping, but developers running `go test ./...` on their host get **zero telemetry**.

This plan introduces Skaffold-inspired declarative pipeline stages in `devx.yaml` and a universal `devx run` command. It embraces our new **Familiarity-First** design principle while resolving edge cases around configuration loading, secret injection, and global flag compliance.

---

## 1. Deep Engineering Refinements (The "Final Polish")

After a thorough engineering review, the following critical improvements have been made to the original design:

### 1.1 Secret Vault Injection (The "Killer Feature")
A major gap in tools like Skaffold or Make is secret management. Since devx already supports secret injection (Idea 30) via the `env:` block in `devx.yaml`, we must wire this into our pipeline explicitly:
- `devx run -- <cmd>` will evaluate and securely inject the `devx.yaml` env block into the child process.
- The pipeline stages (`test`, `lint`, `build`, `verify`) in `devx agent ship` will also inherit these injected secrets.
This means developers can use `devx run` or custom pipelines without needing `.env` wrappers.

### 1.2 Global Flag Compliance
Per the project's strict planning requirements, new commands must honor global flags:
- `--dry-run`: `devx run --dry-run -- make deploy` will print `Would run: make deploy` and exit 0 without executing the child process or emitting telemetry.
- `--json`: `devx run` will suppress non-JSON stdout, and only emit telemetry errors or span results if needed, though typically `run` forwards stdout naturally.

### 1.3 Strict Cobra Argument Parsing 
Using `DisableFlagParsing: true` is blunt and breaks if we ever add local flags to `devx run`. Instead, we will rely on cobra's standard delimiter `--`. 
Example: `devx run --json -- npm test --coverage`
Cobra parses `--json` for devx, and passes everything after `--` to the inner command. We will enforce `Args: cobra.MinimumNArgs(1)`.

### 1.4 Granular Telemetry on Failures
`RecordEvent` spans for `devx run` and explicit pipelines must capture exit codes to ensure the telemetry backend isn't just showing "duration" but actual success rates.
- `devx.exit_code` (e.g., `0`, `1`, `2`)
- `devx.run.command` (e.g., `npm run lint`)

### 1.5 The "Explicit Wins" Contract
If a user defines a `pipeline:` block in `devx.yaml` but only provides `lint:` and `build:`, `devx agent ship` will **not** attempt to auto-detect a `test:` stage. If the block exists, the user owns the pipeline entirely. Auto-detection is strictly an all-or-nothing fallback.

---

## 2. Proposed Changes

### Config Model

#### [MODIFY] `internal/orchestrator/dag.go`
Add pipeline types, including the scaffolded lifecycle hooks (Idea 45.3):
```go
type PipelineStage struct {
    Command  []string   `yaml:"command,omitempty"`
    Commands [][]string `yaml:"commands,omitempty"`
    Before   [][]string `yaml:"before,omitempty"`
    After    [][]string `yaml:"after,omitempty"`
}

func (ps *PipelineStage) Cmds() [][]string {
    if len(ps.Commands) > 0 { return ps.Commands }
    if len(ps.Command) > 0 { return [][]string{ps.Command} }
    return nil
}

type PipelineConfig struct {
    Test   *PipelineStage `yaml:"test,omitempty"`
    Lint   *PipelineStage `yaml:"lint,omitempty"`
    Build  *PipelineStage `yaml:"build,omitempty"`
    Verify *PipelineStage `yaml:"verify,omitempty"`
}

type CustomAction struct {
    Command  []string   `yaml:"command,omitempty"`
    Commands [][]string `yaml:"commands,omitempty"`
}
```
Add `Pipeline *PipelineConfig` and `CustomActions map[string]CustomAction` to the topology struct.

#### [MODIFY] `internal/ci/parser.go`
Ensure the parser reads these new blocks into the config struct safely.

---

### Universal Wrapper

#### [NEW] `cmd/run.go`
```go
var runCmd = &cobra.Command{
    Use:   "run -- [command...]",
    Short: "Run a command with telemetry and secret injection",
    Args:  cobra.MinimumNArgs(1),
    RunE:  runRun,
}
```
Implementation logic:
1. Handle `--dry-run`: Return early logging the intent.
2. Load `devx.yaml` secrets if present (via `envvault`).
3. Set up `os/exec.Command` with the merged environment.
4. Execute and time the result.
5. `telemetry.RecordEvent("devx_run", duration, exitCode, cmd...)`.
6. Return `fmt.Errorf("")` with the exact exit code so the CLI exits correctly.

---

### Ship Integration

#### [MODIFY] `cmd/agent_ship.go`
Load `devx.yaml` (if present) before preflight, resolve environment vaults, and pass both the environment and the `Pipeline` config to `ship.RunPreFlight`.

#### [MODIFY] `internal/ship/ship.go`
Refactor `RunPreFlight`:
```go
func RunPreFlight(dir string, verbose bool, pipeline *orchestrator.PipelineConfig, env []string) (*PreFlightResult, error) {
    if pipeline != nil {
        return runExplicitPipeline(dir, verbose, pipeline, env)
    }
    // ... fallback to auto-detection (which is also given env vars) ...
}
```

---

### Documentation & Dogfooding

#### [MODIFY] `devx.yaml` (Root directory)
Implement our own pipeline:
```yaml
name: devx
pipeline:
  test:
    command: ["go", "test", "./..."]
  lint:
    command: ["go", "vet", "./..."]
  build:
    command: ["go", "build", "./..."]
```

#### [MODIFY] `docs/guide/introduction.md`
Add the Familiarity-First design principle.

#### [MODIFY] `docs/guide/pipeline.md` (NEW)
Create documentation dedicated to the pipeline configuration, explaining stages, multi-commands, hooks (as coming soon), and `devx run`. This replaces updating `caching.md`.

#### [MODIFY] `devx.yaml.example` & `FEATURES.md`
Update with the new features.

---

## 3. Gap Analysis

| Weakness Avoided | Improvement Taken |
|------------------|-------------------|
| Misplaced documentation | Created a new `pipeline.md` guide specifically for orchestrations. |
| Flag parameter ingestion | Reliant on standard `--` delimiter via Cobra args instead of brute-force disable. |
| Telemetry lacks failure context | Integrated `devx.exit_code` as an explicit metric. |
| Missing secrets | Brought `envvault` secret injection into the `devx run` lifecycle. |
| Half-baked explicit pipelines | Instituted the "Explicit Wins" rule: auto-detection never selectively fills gaps in explicit pipelines. |

## 4. Verification Plan

- `go vet ./...` & `go test ./...`
- `cmd/run_test.go` — Test telemetry spans, secret injection passing, and exit code propagation.
- **Dogfooding Check:** Apply `devx.yaml` to the root project, spin up Grafana (`devx trace spawn grafana`), execute `devx run -- go test ./...`, and check the dashboard.
- Finally, use `devx agent ship` to push, proving the pipeline parser executed our `devx.yaml` override.
