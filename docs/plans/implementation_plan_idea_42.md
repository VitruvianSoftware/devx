# devx ci run — Local CI Pipeline Emulation (Idea 42)

Solve the "fix ci... fix ci again..." commit loop by executing GitHub Actions `run:` steps locally inside isolated Podman containers with 80% parity. Matrix jobs run in parallel to faithfully mimic the real pipeline topology.

## Scope & Explicit Limitations

> [!WARNING]
> **`uses:` actions are intentionally unsupported.** This tool executes only `run:` shell blocks. Composite/JS actions like `actions/upload-artifact`, `golangci/golangci-lint-action`, or `actions/setup-go` are **skipped with a visible warning** in both CLI output and `--json` mode. This limitation will be documented in the CLI `--help`, inline code comments, and the official VitePress docs page. The rationale: emulating `uses:` faithfully is why `nektos/act` is 50K+ lines and still broken — we trade completeness for reliability.

**What IS supported (the 80%):**
- `run:` shell blocks (the actual developer logic)
- `strategy.matrix` expansion with parallel execution
- `needs:` job dependency ordering (DAG)
- `env:` at workflow, job, and step levels
- `if:` conditionals (simple expression evaluation)
- `working-directory:` on steps
- `shell:` specification (bash/sh)
- `services:` containers (mapped to `internal/testing/ephemeral.go` patterns)
- `${{ secrets.X }}` injection from devx Vault providers
- `${{ matrix.X }}` and `${{ env.X }}` template substitution
- `continue-on-error:` and `timeout-minutes:`

**What is NOT supported:**
- `uses:` composite/JS actions (skipped with warning)
- `${{ steps.id.outputs.X }}` (step output capture)
- `${{ github.* }}` / `${{ runner.* }}` context objects (stubbed with sensible defaults)
- Functions like `contains()`, `startsWith()`, `hashFiles()`

---

## Proposed Changes

### Phase 1: Parsing Engine

#### [NEW] [internal/ci/parser.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/parser.go)

Unmarshals `.github/workflows/*.yml` into typed Go structs. Core responsibilities:

- **Workflow struct**: Top-level `name`, `env`, `jobs` map.
- **Job struct**: `runs-on`, `needs`, `strategy.matrix`, `services`, `env`, `if`, `steps`, `timeout-minutes`, `continue-on-error`.
- **Step struct**: `name`, `run`, `uses`, `env`, `if`, `working-directory`, `shell`, `continue-on-error`, `timeout-minutes`.
- **Matrix expansion**: `strategy.matrix` with `include`/`exclude` modifiers → expands a single job definition into N concrete `ExpandedJob` structs, each carrying the resolved matrix values. Example: `matrix: {goos: [darwin, linux], goarch: [amd64, arm64]}` → 4 expanded jobs.
- **Job DAG resolution**: Parses `needs:` into a dependency graph and produces execution tiers via Kahn's algorithm (reusing the pattern from `internal/orchestrator/dag.go`). Jobs in the same tier run in parallel; subsequent tiers wait for all predecessors.

#### [NEW] [internal/ci/template.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/template.go)

Handles `${{ ... }}` expression evaluation. This is **not** a full GitHub expression evaluator — it's a targeted regex-based substitution engine with a defined scope:

| Expression | Strategy |
|---|---|
| `${{ env.VAR }}` | Literal replacement from merged env map |
| `${{ secrets.VAR }}` | Lookup from devx Vault (1Password/Bitwarden/GCP) via `internal/envvault` |
| `${{ matrix.VAR }}` | Literal replacement from the current expanded matrix row |
| `${{ github.event_name }}` | Stubbed to `"push"` (local always simulates push) |
| `${{ runner.os }}` | Stubbed to container OS (`"Linux"`) |
| `${{ runner.arch }}` | Stubbed to host arch |
| Unknown expressions | Left as-is with a debug warning |

For `if:` conditionals: evaluate simple truthy/falsy checks (`if: ${{ matrix.goos == 'linux' }}`). Complex expressions with functions (`contains()`, `hashFiles()`) will be treated as truthy (run the step) with a warning. This is the pragmatic "fail-open" approach — better to run a step unnecessarily than skip it silently.

#### [NEW] [internal/ci/parser_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/parser_test.go)
- Parse the real `devx/.github/workflows/ci.yml`.
- Test matrix expansion: 2×2 matrix → 4 expanded jobs.
- Test `needs:` DAG: `build` depends on `[lint, test]` → 2 tiers.
- Test `if:` conditional evaluation for simple equality checks.

---

### Phase 2: Execution Engine

#### [NEW] [internal/ci/executor.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/executor.go)

Core execution loop:

1. **Image resolution**: Check for `devcontainer.json` via `internal/devcontainer.Load()`. If found, use its `Image` field. Otherwise, default to `ubuntu:latest` and warn the user about potential tool gaps.
2. **Services provisioning**: If the job declares `services:` (e.g., `postgres:15`), spin them up using the same ephemeral container pattern from `internal/testing/ephemeral.go` — random ports, health-wait, automatic teardown.
3. **Container lifecycle**: One container per **job** (not per step). Steps execute sequentially inside the same container via `podman exec`. This preserves filesystem state between steps (e.g., step 1 installs Go, step 2 runs `go test`). The workspace is bind-mounted at `/workspace` via `-v $PWD:/workspace -w /workspace`.
4. **Step execution**: For each step:
   - If `uses:` → log a visible `⚠️  SKIPPED (uses: action)` warning and continue.
   - If `run:` → template-substitute expressions, then execute via `podman exec -w <working-directory> <container> <shell> -c "<run block>"`.
   - Honor `continue-on-error:` (don't fail the job on step failure).
   - Honor `timeout-minutes:` via `context.WithTimeout`.
5. **Exit code propagation**: If any non-`continue-on-error` step fails, the job fails. The overall exit code is non-zero if any job fails.

#### Parallel Output Strategy

> [!IMPORTANT]
> The previous plan proposed a Bubble Tea TUI for parallel output. This is **wrong**. A full interactive TUI would break `--json`, `--non-interactive`, piping to files, and AI agent consumption. Instead:

**Default mode (TTY):** Goroutine-safe prefixed line streaming, identical to Docker Compose's output model. Each parallel job gets a color-coded prefix:

```
[lint]          | golangci-lint run ./...
[test]          | go test -v -race ./...
[build·dar·a64] | go build -ldflags="-s -w" -o devx .
[build·lin·a64] | go build -ldflags="-s -w" -o devx .
```

A synchronized `io.Writer` wrapper ensures lines don't interleave mid-line. The prefix includes the job name and, for matrix jobs, the condensed matrix values.

**JSON mode (`--json`):** Structured JSON Lines emitted per-step with `job`, `step`, `status`, `duration`, and `output` fields.

#### [NEW] [internal/ci/writer.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/writer.go)
- `PrefixedWriter`: Thread-safe line-buffered writer that prepends `[job-name]` with Lipgloss color coding.
- Handles the synchronized output multiplexing for parallel goroutines.

---

### Phase 3: CLI Commands

#### [NEW] [cmd/ci.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/ci.go)
- Root `devx ci` command group.

#### [NEW] [cmd/ci_run.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/ci_run.go)

```
devx ci run [workflow.yml] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--job` | (all) | Run only specific job(s) by name |
| `--image` | (auto) | Override the container image |
| `--runtime` | `podman` | Container runtime (`podman` or `docker`) |
| `--json` | false | Structured JSON Lines output |
| `--dry-run` | false | Parse and display execution plan without running |
| `-y` | false | Non-interactive (skip prompts) |

**Workflow discovery:** If no argument is provided, scans `.github/workflows/` and lists available workflows with a `huh.Select` picker (or runs the first one found in `-y` mode).

**Dry-run output:** Shows the parsed DAG, expanded matrix, and the exact `run:` blocks that would execute — extremely useful for agents and debugging.

---

### Phase 4: Documentation

#### [NEW] [docs/guide/ci.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/ci.md)
- Usage examples, flags, and limitations.
- Explicit section titled **"What `devx ci run` Does NOT Do"** documenting the `uses:` limitation prominently.
- Comparison table vs `nektos/act`.

#### [MODIFY] [docs/.vitepress/config.mjs](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/.vitepress/config.mjs)
- Add `{ text: 'Local CI Emulation', link: '/guide/ci' }` to sidebar.

#### [MODIFY] [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md)
- Migrate Idea 42 from `IDEAS.md` to `FEATURES.md` after shipping.

#### [MODIFY] [README.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/README.md)
- Add a bullet to the "Why devx?" section.

---

## Verification Plan

### Automated Tests
- `internal/ci/parser_test.go`: Parse the real `devx` CI workflow, verify matrix expansion and DAG ordering.
- `internal/ci/template_test.go`: Test expression substitution for `env`, `secrets`, `matrix`, and `if:` conditionals.
- `go build ./... && go vet ./... && go test ./...`

### Manual Verification
- Run `devx ci run ci.yml --dry-run` on the devx repo — verify it correctly parses the 4-job structure (`lint`, `test`, `build` with 2×2 matrix, `validate-template`).
- Run `devx ci run ci.yml --job test` — verify `go test -v -race -coverprofile=coverage.out ./...` actually executes inside a container.
- Run `devx ci run ci.yml --json` — verify structured output is valid JSON Lines.

### Documentation (Mandatory)
- Create `docs/guide/ci.md`, register in sidebar, push and verify docs deploy.
