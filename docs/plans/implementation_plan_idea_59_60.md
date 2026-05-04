# Implementation Plan: Idea 59 (Intelligent Failure Recovery) & Idea 60 (Natural Language DB Queries)

**Date:** 2026-05-04
**Idea/Issue References:** Idea 59, Idea 60

---

## 1. Deep Research & Context

* **Objective:** Build two AI-powered features on the existing `internal/ai` foundation: (1) automatic failure diagnosis across all `devx` commands, and (2) natural language database queries via `devx db ask`.
* **Sibling Implementations Studied:**
  - `internal/devxerr/recovery.go` — Existing pattern-match recovery (`RecoverGcloudAuth`). Our rule-based fallback follows this exact pattern.
  - `cmd/db_synthesize.go` — The closest sibling for `devx db ask`. Follows the: validate engine → check container → discover AI → extract schema → prompt → execute pipeline.
  - `internal/database/synthesizer.go` — `ExtractSchema()` and `PipeSQL()` are directly reusable for `db ask`.
  - `internal/logs/crashlog.go` — `TailContainerCrashLogs()` pattern for collecting crash context.
  - `cmd/root.go:Execute()` — The centralized error handler where we intercept `DevxError` before `os.Exit()`.
* **Gap Analysis:**
  - `Execute()` in `root.go` currently prints the raw error and exits. There is **no hook point** for post-failure analysis. We need to add one without disrupting existing exit code behavior.
  - `db_synthesize.go` uses `ai.GenerateCompletion()` directly. For `db ask`, we need the same provider discovery but with a different system prompt and read-only SQL execution.
  - The existing `RecoverGcloudAuth()` in `recovery.go` only handles one specific pattern. Our rule-based engine generalizes this into a pattern table.

## 2. Design Decisions

### Design Principles Alignment

* [x] **One CLI, everything** — Both features live inside `devx`, not as external tools.
* [x] **Convention over configuration** — Zero-config. AI diagnosis activates automatically when a provider is detected. `db ask` canned queries require no AI at all.
* [x] **Transparency** — The diagnosis prints exactly what context was collected and what the AI concluded. `db ask` shows the generated SQL before executing via `--dry-run`.
* [x] **Idempotency** — Both features are read-only. Diagnosis never mutates state. `db ask` runs inside `SET TRANSACTION READ ONLY` by default.
* [x] **AI-native** — Both support `--json` for structured output. `db ask` supports `--dry-run` to preview generated SQL.
* [x] **Optimized Inner Loop** — Eliminates the #1 time sink (debugging crashes) and the #2 context-switch (opening database GUIs).

### Non-Obvious Choices

1. **Diagnosis hook in `Execute()`, not in individual commands.** Alternative: add `defer diagnose()` to every `RunE`. Rejected because it requires modifying 30+ command files and is fragile (new commands forget to add it). The centralized `Execute()` intercept catches everything, including errors from commands we haven't written yet.

2. **Rule-based engine as the _baseline_, not the AI.** The design constraint from IDEAS.md is critical: "The command must never print 'Error: no AI provider found.'" The rule-based fallback works without any AI. The LLM is an _enhancement_ that activates silently when available.

3. **Read-only transactions for `db ask` — enforced at the SQL level, not just by prompt engineering.** We wrap all AI-generated SQL in `SET TRANSACTION READ ONLY` regardless of what the LLM produces. The `--allow-writes` flag explicitly opts out with a confirmation gate.

4. **`db ask` canned queries work without AI.** The `--recent`, `--sizes`, `--missing-indexes`, and `--nulls` flags execute deterministic SQL snippets directly. This means `db ask` is useful even in environments with zero AI capability.

## 3. Proposed Changes

### Component 1: Intelligent Failure Recovery (Idea 59)

---

#### `[NEW]` [diagnose.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ai/diagnose.go)

Core diagnosis engine with two tiers:

```go
// DiagnoseFailure attempts to explain a devx command failure.
// Returns a human-readable diagnosis string, or "" if no diagnosis is available.
func DiagnoseFailure(cmdName string, exitCode int, errMsg string, stderr string) string
```

**Rule-based tier (`matchRule`):**
A table of `diagnosisRule` structs, each with:
- `Pattern string` — substring or regex to match against the error/stderr
- `ExitCode int` — optional exit code to match (0 = match any)
- `Diagnosis string` — human-readable explanation
- `Suggestion string` — actionable fix command

Initial rules (~15):
| Pattern | Diagnosis | Suggestion |
|---------|-----------|------------|
| `password authentication failed` | Database credentials mismatch between container env and app env | `devx config pull` to sync credentials |
| `address already in use` | Port conflict — another process occupies the port | `lsof -i :<port>` to find the process |
| `connection refused` on a DB port | Database container not running or not ready yet | `devx db spawn <engine>` |
| `no such container` | Container was removed or never created | `devx up` to recreate |
| `ECONNREFUSED 127.0.0.1:11434` | Ollama not running | `ollama serve` |
| `certificate has expired` | Cloudflare or TLS cert expiry | `devx doctor auth` |
| `context deadline exceeded` | Container healthcheck timeout | Check `healthcheck.timeout` in devx.yaml |
| `image not found` / `manifest unknown` | Container image doesn't exist in registry | Verify image name in devx.yaml |
| `permission denied` | File permission or RBAC issue | Check file ownership / `devx bridge rbac` |
| `gcloud.auth.docker-helper` | Expired GCP auth token | `gcloud auth login` |
| `OOMKilled` | Container ran out of memory | Increase VM memory via `devx vm resize` |
| `exec format error` | Architecture mismatch (arm64 vs amd64) | Use `--platform` flag or multi-arch image |
| `devx-db-` + `not running` | Database container not started | `devx db spawn <engine>` |
| `CF_TUNNEL_TOKEN` | Missing Cloudflare tunnel token | `devx doctor auth` |
| `admin:org` | Missing GitHub scope | `gh auth refresh -s admin:org` |

**LLM tier (`aiDiagnose`):**
- Collects runtime context: `podman ps -a --format json`, last 30 lines of logs from crashing containers, and environment variable keys (values redacted to `***`).
- Builds a structured prompt with the error, context, and `devx.yaml` topology.
- Calls `ai.RunAgentPrompt()` (which handles the ollama launch → chat API → none cascade).
- Returns the AI's diagnosis, or falls through silently if no AI is available.

#### `[MODIFY]` [root.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/root.go)

Modify `Execute()` (lines 91-101) to invoke the diagnosis engine before exiting:

```go
func Execute() {
    if err := rootCmd.Execute(); err != nil {
        var dex *devxerr.DevxError
        exitCode := 1
        if errors.As(err, &dex) {
            exitCode = dex.ExitCode
        }
        _, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

        // Idea 59: Attempt intelligent failure diagnosis
        if !outputJSON {
            diagnosis := ai.DiagnoseFailure(os.Args[1], exitCode, err.Error(), "")
            if diagnosis != "" {
                _, _ = fmt.Fprintf(os.Stderr, "\n%s\n", diagnosis)
            }
        }

        os.Exit(exitCode)
    }
}
```

The JSON mode suppression is intentional — AI agents parsing `--json` output shouldn't get free-form diagnosis text injected into their structured pipeline.

#### `[MODIFY]` [error.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/devxerr/error.go)

Add new exit codes for the diagnosis and db ask features:

```go
// Idea 59: Failure Recovery
CodeDiagnosisTimeout = 90 // AI diagnosis timed out

// Idea 60: Natural Language DB Queries
CodeDBAskNoAI         = 91 // No AI provider for natural language query
CodeDBAskQueryFailed  = 92 // Generated SQL failed to execute
CodeDBAskReadOnly     = 93 // Write attempted without --allow-writes
```

---

### Component 2: Natural Language Database Queries (Idea 60)

---

#### `[NEW]` [db_ask.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/db_ask.go)

New Cobra command following `db_synthesize.go` patterns exactly:

```
devx db ask <engine> "<question>"     # Natural language query
devx db ask <engine> --recent         # Canned: last 10 rows per table
devx db ask <engine> --sizes          # Canned: table sizes
devx db ask <engine> --missing-indexes # Canned: tables without indexes
devx db ask <engine> --nulls <table>  # Canned: columns with high NULL ratios
```

**Pipeline:**
1. Validate engine is synthesizable (postgres/mysql)
2. Check container is running via `podman inspect`
3. **Canned query path** (no AI needed): if `--recent`/`--sizes`/`--missing-indexes`/`--nulls` is passed, execute the hardcoded SQL directly and render as a table
4. **NL query path**: discover AI provider → extract schema → build prompt → generate SQL → sanitize → dry-run gate → execute in read-only transaction → render results as table
5. `--dry-run`: shows the generated SQL without executing
6. `--allow-writes`: removes the `SET TRANSACTION READ ONLY` wrapper (with confirmation)
7. `--json`: outputs results as JSON array
8. `--model`: overrides the LLM model

**System prompt for text-to-SQL:**
```
You are an expert SQL developer. Given the database schema below, translate the user's natural language question into a single valid SQL SELECT query. Rules:
1. Output ONLY the raw SQL query. No markdown, no explanations, no code blocks.
2. Use only tables and columns that exist in the schema.
3. Prefer explicit column names over SELECT *.
4. Use appropriate JOINs when the question involves multiple tables.
5. Add LIMIT 100 unless the user explicitly asks for all results.
```

#### `[NEW]` [query.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/database/query.go)

Contains:

1. **`CannedQueries`** — a map of query name → engine-specific SQL:
   - `recent`: `SELECT * FROM <table> ORDER BY ctid DESC LIMIT 10` per table (postgres) / `LIMIT 10` (mysql)
   - `sizes`: `SELECT relname, pg_size_pretty(pg_total_relation_size(oid)) FROM pg_class WHERE relkind='r'` (postgres) / `SELECT table_name, data_length FROM information_schema.tables` (mysql)
   - `missing-indexes`: Tables with no non-primary indexes
   - `nulls`: Per-column NULL ratio for a given table

2. **`ExecuteReadOnlyQuery(runtime, engine, sql string) ([][]string, []string, error)`**
   Returns rows and column headers. Wraps in `SET TRANSACTION READ ONLY; BEGIN; <sql>; COMMIT;` for safety.

3. **`RenderTable(headers []string, rows [][]string)`**
   Renders results using lipgloss table formatting (consistent with the crash log box styling from `crashlog.go`).

### Architecture & Runtime Compliance Checklist

* [x] **Provider Abstraction:** `db ask` uses the `runtime` flag (`--runtime podman|docker`) consistent with `db_synthesize.go`. Does NOT use `provider.ContainerRuntime` because it follows the simpler string-based pattern of all `db` subcommands.
* [x] **Config Cascade:** Not applicable — these features don't read `devx.yaml` (they operate on running containers directly).
* [x] **Global Flags:**
  - `--dry-run`: Shows generated SQL/diagnosis without executing. ✓
  - `-y`: Skips `--allow-writes` confirmation prompt. ✓
  - `--json`: Outputs structured JSON. ✓
  - `--provider`: Not applicable (uses `--runtime` per db subcommand pattern). ✓
* [x] **Error Handling:** Fail-fast. Diagnosis never blocks the original exit code. `db ask` returns specific exit codes.
* [x] **Environment & Context:** All commands execute on the host. Container interaction is via `podman exec`.
* [x] **Pre-push Hook / `devx audit`:** No impact. These features don't modify exit codes or scanning behavior.

## 4. Configuration Schema

No new `devx.yaml` or `cluster.yaml` keys. No new environment variables. No new CLI flags on existing commands (only new commands).

## 5. Documentation Ecosystem (Mandatory)

### Official Docs & CLI
* [ ] **Official Docs:** Create `docs/guide/failure-recovery.md` and add a section to `docs/guide/databases.md` for `db ask`. Wire into `docs/.vitepress/config.mjs` sidebar.
* [ ] **CLI Help Text:** Full `Use`, `Short`, `Long`, `Example` fields for `db ask`. Updated `rootCmd.Long` to mention AI-powered diagnosis.
* [ ] **Feature Ecosystem (`cmd/root.go`):** No change needed — diagnosis is invisible (it activates on errors, not as a command).
* [ ] **Environment Health (`devx doctor`):** No new dependencies.
* [ ] **Main README:** Add `devx db ask` to the CLI reference table.
* [ ] **Feature Trackers:** Migrate Ideas 59 & 60 from `IDEAS.md` to `FEATURES.md`. Bump counter.
* [ ] **CHANGELOG:** Auto-generated via conventional commits.

### Agent Skills Templates (Mandatory Audit)
* [ ] All 12 skill template files — add `devx db ask` usage patterns and exit codes.

### Example Configs & Templates
* [ ] `devx.yaml.example` — No changes (no new YAML keys).

## 6. Verification Plan

### Automated Tests

```bash
# 1. Build
go build -o devx main.go

# 2. Test rule-based diagnosis (no AI needed)
./devx db spawn nonexistent-engine 2>&1  # Should trigger diagnosis hint

# 3. Test db ask canned queries (no AI needed)
./devx db ask postgres --sizes
./devx db ask postgres --recent
./devx db ask postgres --missing-indexes

# 4. Test db ask with AI (requires local ollama)
./devx db ask postgres "show me all tables and their row counts"
./devx db ask postgres "show me all tables" --dry-run

# 5. Test read-only safety
./devx db ask postgres "DROP TABLE users"  # Should fail (read-only)
./devx db ask postgres --allow-writes "INSERT INTO..." # Should prompt

# 6. Test JSON output
./devx db ask postgres --sizes --json

# 7. Lint
golangci-lint run ./...
```

### Edge Cases
- Container not running → clear error message pointing to `devx db spawn`
- No AI provider for NL query → specific exit code, no crash
- LLM generates invalid SQL → show error + generated SQL for debugging
- LLM generates markdown-wrapped SQL → `SanitizeLLMSQL` strips it (reuse existing function)
- `--allow-writes` without `-y` → confirmation prompt
- Empty result set → display "(0 rows)" message

### Manual Verification
- Run `devx up` with a deliberately broken `.env` (wrong password) and verify the diagnosis output explains the credential mismatch
- Run `devx db ask postgres "users created today"` against a real seeded database and verify correct results

## Open Questions

> [!IMPORTANT]
> **Table rendering library:** Should we use lipgloss `table` package (already a dependency via charmbracelet) for `db ask` results, or a simpler `text/tabwriter`? Lipgloss table gives prettier output but adds complexity. I'm leaning toward lipgloss for consistency with the rest of the TUI.

> [!NOTE]
> **Diagnosis timeout:** The AI diagnosis runs _after_ the command fails, which means the developer is already waiting. I've defaulted to a 15-second timeout on the AI call to avoid making a bad experience worse. If the AI takes longer, we silently skip it. Is 15s acceptable, or should this be configurable?
