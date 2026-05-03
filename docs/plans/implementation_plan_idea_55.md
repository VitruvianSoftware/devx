# Instant PR Sandboxing (`devx preview`)

Implementation of Idea 55. Creates an isolated sandbox for reviewing a PR without disrupting the developer's active branch, databases, or tunnel state.

## Design Decisions Confirmed

- **Database Namespacing**: `--project` flag on `devx db spawn`/`devx db rm`. If provided → `devx-db-<project>-<engine>` / `devx-data-<project>-<engine>`. If omitted → legacy `devx-db-<engine>`. `devx up` only passes `--project` when `DEVX_PROJECT_OVERRIDE` is set.
- **Automatic Teardown**: `SIGINT`/`SIGTERM` trap cleans up worktree, databases, tunnel, DNS, and exposure store entries.

## Gap Analysis (from review)

| Gap | Resolution |
|---|---|
| Cloudflare tunnel + exposure store cleanup on teardown | Teardown calls `cloudflare.DeleteTunnel`, `cloudflare.RemoveDNS`, and `exposure.RemoveByName` |
| Global flag compliance (`--dry-run`, `--json`, `-y`) | Explicitly handled in `cmd/preview.go` and threaded through to subprocess |
| `gh` CLI prerequisite | Fail-fast with actionable error; update `doctor/check.go` FeatureArea |
| Package structure (`cmd/` only vs `internal/`) | New `internal/preview/` package holds orchestration logic |
| `devx nuke` integration | `nuke.Collect` extended to discover `devx-db-pr-*` containers and stale worktrees |
| Fork PRs | Documented limitation — same-repo PRs only in v1 |
| Concurrent previews | Port conflicts handled by existing `network.ResolvePort`; tunnel names namespaced by PR number |

---

## Proposed Changes

### Preview Orchestration (new package)

#### [NEW] [internal/preview/sandbox.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/preview/sandbox.go)

Core orchestration logic, separated from the thin `cmd/` wrapper per established codebase convention.

- **`type Sandbox struct`**: Holds PR number, worktree path, branch name, project override name, tunnel name.
- **`func New(prNumber int) *Sandbox`**: Computes derived names (`devx-pr-<N>`, `pr-<N>`).
- **`func (s *Sandbox) Setup() error`**:
  1. Validates `gh` is installed and authenticated — fails with `"Install GitHub CLI: brew install gh && gh auth login"`.
  2. `gh pr view <N> --json headRefName` to fetch the branch name.
  3. `git fetch origin pull/<N>/head:devx-pr-<N>`.
  4. `git worktree add <worktreeDir> devx-pr-<N>` where `worktreeDir` = `os.TempDir()/devx-preview-<N>`.
  5. Verifies a `devx.yaml` exists in the worktree root.
- **`func (s *Sandbox) Run(ctx context.Context) error`**:
  1. Resolves `devx` binary path via `os.Executable()`.
  2. Builds subprocess: `devx up` inside worktree dir with `DEVX_PROJECT_OVERRIDE=pr-<N>` injected into env.
  3. Forwards global flags (`--json`, `-y`, `--env-file`) to the subprocess.
  4. Blocks until subprocess exits or context cancelled.
- **`func (s *Sandbox) Teardown() error`**:
  1. Reads the worktree's `devx.yaml` to discover database engines.
  2. For each engine: `devx db rm <engine> --project pr-<N> -y`.
  3. Calls `cloudflare.DeleteTunnel(tunnelName)` to remove the Cloudflare tunnel.
  4. Calls `exposure.RemoveByName(tunnelName)` to purge the exposure store.
  5. `git worktree remove --force <worktreeDir>`.
  6. `git branch -D devx-pr-<N>`.
  7. All errors are collected and reported, not fatal — best-effort cleanup.
- **`func (s *Sandbox) DryRun() string`**: Returns a human-readable summary of what *would* happen without executing.
- **`func (s *Sandbox) JSON() map[string]any`**: Returns structured output for `--json`.

#### [NEW] [internal/preview/sandbox_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/preview/sandbox_test.go)

- **`TestNew_DerivedNames`**: Verifies branch name, worktree path, and project override are computed correctly.
- **`TestDryRun_Output`**: Verifies dry-run output contains worktree path, DB names, and branch name.
- **`TestJSON_Structure`**: Verifies JSON output contains all expected keys.

---

### CLI Command

#### [NEW] [cmd/preview.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/preview.go)

Thin wrapper following the `cmd/db_spawn.go` → `internal/database/` pattern.

- Registers `devx preview <PR_NUMBER>` under `GroupID: "orchestration"`.
- `Use: "preview <pr-number>"`, `Short: "Spin up an isolated sandbox to review a PR without switching branches"`.
- **`RunE`**:
  1. Parses `PR_NUMBER` from `args[0]` (validates integer).
  2. Calls `preview.New(prNumber)`.
  3. If `DryRun` → calls `sandbox.DryRun()`, prints, returns.
  4. If `outputJSON` → calls `sandbox.JSON()`, marshals, returns.
  5. Calls `sandbox.Setup()`.
  6. Registers `SIGINT`/`SIGTERM` signal handler → calls `sandbox.Teardown()`.
  7. Calls `sandbox.Run(ctx)`.
  8. Calls `sandbox.Teardown()` (normal exit path).
- `init()`: `rootCmd.AddCommand(previewCmd)`.

---

### Database Isolation

#### [MODIFY] [db_spawn.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/db_spawn.go)

- Add `var spawnProject string` flag: `--project` (string, default `""`).
- Update `containerName` and `volumeName` (lines 79–80):
  ```go
  containerName := fmt.Sprintf("devx-db-%s", engineName)
  volumeName := fmt.Sprintf("devx-data-%s", engineName)
  if spawnProject != "" {
      containerName = fmt.Sprintf("devx-db-%s-%s", spawnProject, engineName)
      volumeName = fmt.Sprintf("devx-data-%s-%s", spawnProject, engineName)
  }
  ```
- Register flag in `init()`: `spawnCmd.Flags().StringVar(&spawnProject, "project", "", "Namespace isolation prefix for containers and volumes")`.

#### [MODIFY] [db_rm.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/db_rm.go)

- Add `var rmProject string` flag: `--project`.
- Update `containerName` and `volumeName` (line 61–62) with same conditional logic as spawn.
- Register flag in `init()`.

#### [MODIFY] [up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go)

- After `projectName` resolution (line 76–79), add:
  ```go
  if override := os.Getenv("DEVX_PROJECT_OVERRIDE"); override != "" {
      projectName = override
  }
  ```
- In the database spawn loop (line 102–105), if `DEVX_PROJECT_OVERRIDE` is set, append `"--project", projectName` to the `args` slice.

---

### Nuke Integration

#### [MODIFY] [internal/nuke/nuke.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/nuke/nuke.go)

- Add `collectPreviewArtifacts()` call in `Collect()` (after line 68).
- **`func (m *Manifest) collectPreviewArtifacts()`**:
  - Lists containers matching `devx-db-pr-*` pattern via `runtime.Exec("ps", "-a", "--filter", "name=devx-db-pr-", "--format", "{{.Names}}")`.
  - Lists volumes matching `devx-data-pr-*` pattern.
  - Lists git worktrees via `git worktree list --porcelain`, filters paths containing `devx-preview-`.
  - Adds each as an `Item` with `Category: "preview"`.

---

### Doctor Prerequisite Update

#### [MODIFY] [internal/doctor/check.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/doctor/check.go)

- Update the existing GitHub CLI entry (line 112–118) to change `FeatureArea` from `"Sites"` to `"Sites, Preview"` so `devx doctor` output reflects that `gh` is also required for `devx preview`.

---

### Documentation

#### [NEW] [docs/guide/preview.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/preview.md)

New guide page following the pattern of [orchestration.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/orchestration.md). Sections:

1. **What is PR Sandboxing?** — Problem statement (context switching destroys flow).
2. **Quick Start** — `devx preview 42` one-liner with expected output.
3. **How It Works** — Git worktree, database isolation, tunnel namespacing, auto-teardown.
4. **Prerequisites** — `gh` CLI, authenticated GitHub.
5. **Limitations** — Fork PRs unsupported in v1, one preview at a time recommended.
6. **Related Commands** — Links to `devx up`, `devx db spawn`, `devx nuke`.

#### [MODIFY] [docs/.vitepress/config.mjs](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/.vitepress/config.mjs)

Add `{ text: 'PR Preview Sandbox', link: '/guide/preview' }` to the **Orchestration & State** sidebar section (after line 63, before `Diagnostics & State`).

#### [MODIFY] [docs/guide/journeys.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/journeys.md)

Add a new **User Journey #5: The Code Reviewer** section at the end:

```markdown
## 5. The Code Reviewer: Instant PR Sandboxing

### Step 1: Preview the PR
```bash
devx preview 42
```
This creates an isolated worktree, provisions namespaced databases, and exposes the PR's services on unique tunnel URLs — all without touching your current branch.

### Step 2: Review and Test
Open the PR's tunnel URL in your browser, run manual tests, or execute the PR's test suite from the worktree.

### Step 3: Exit
Press `Ctrl+C` to tear down the sandbox. The worktree, databases, and tunnels are cleaned up automatically.
```

#### [MODIFY] [README.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/README.md)

- Add to the **Orchestration & State** feature list (after line 44):
  ```markdown
  * 🔍 **Instant PR Sandboxing:** Review any PR without switching branches. `devx preview 42` creates an isolated worktree with dedicated databases and tunnel URLs, then cleans up automatically on exit.
  ```
- Add to the **Prerequisites table** (line 113):
  ```markdown
  | [gh](https://cli.github.com) | `brew install gh` | GitHub CLI (for `devx sites`, `devx preview`) |
  ```

#### [MODIFY] [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md)

Migrate Idea 55 from `IDEAS.md` to `FEATURES.md` after implementation is verified.

#### [MODIFY] [IDEAS.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/IDEAS.md)

Mark Idea 55 with a `✅ Shipped →` prefix and link to `FEATURES.md` entry, following the existing convention.

---

### CLI Help Grouping

#### [MODIFY] [cmd/root.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/root.go)

The `preview` command registers under `GroupID: "orchestration"` which already exists (line 132). No changes needed to root.go group definitions. The command will naturally appear in the **Orchestration & State** section of `devx --help`.

---

## Verification Plan

### Automated Tests

```bash
# Full build verification
go build ./...

# Full test suite (existing + new)
go test ./... -count=1

# Targeted new test coverage
go test ./internal/preview/... -v -count=1
```

### Edge Case Scenarios

| Scenario | Expected Behavior |
|---|---|
| `gh` not installed | Fail-fast: `"GitHub CLI is required for devx preview. Install: brew install gh"` |
| `gh` not authenticated | Fail-fast: `"GitHub CLI is not authenticated. Run: gh auth login"` |
| Invalid PR number (string) | Cobra `ExactArgs(1)` + `strconv.Atoi` error: `"invalid PR number"` |
| PR doesn't exist | `gh pr view` exits non-zero → `"PR #999 not found in this repository"` |
| PR from a fork | `git fetch` succeeds but branch may not resolve → fail with actionable message |
| Port conflict with active `devx up` | `network.ResolvePort` auto-shifts; `devx db spawn` interactive bump (bypassed with `-y`) |
| `--dry-run` | Prints worktree path, DB names, branch, tunnel name — no side effects |
| `--json` | Emits `{"pr": 42, "worktree": "/tmp/...", "branch": "...", "databases": [...]}` |
| `-y` | Suppresses all interactive prompts during spawn/rm |
| `Ctrl+C` during setup | Teardown runs: worktree removed, partial DBs cleaned, branch deleted |
| `devx nuke` after crashed preview | Discovers orphaned `devx-db-pr-*` containers and stale worktrees |

### Build Validation

```bash
# License check
mage licensecheck

# Lint
golangci-lint run ./...

# Docs build
cd docs && npm run build
```

### Documentation Updates (post-verification)

- [ ] `docs/guide/preview.md` — new guide page
- [ ] `docs/.vitepress/config.mjs` — sidebar entry
- [ ] `docs/guide/journeys.md` — User Journey #5
- [ ] `README.md` — feature bullet + prerequisite table
- [ ] `FEATURES.md` — migrate Idea 55
- [ ] `IDEAS.md` — mark as shipped
