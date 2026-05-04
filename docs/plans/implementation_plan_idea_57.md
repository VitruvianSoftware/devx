# Implementation Plan: AI-Driven Synthetic Data Generation (Idea 57)

**Date:** 2026-05-03
**Idea/Issue Reference:** Idea 57 (`devx db synthesize`)

---

## 1. Deep Research & Context

* **Objective:** Enable developers to generate highly realistic, edge-case-heavy synthetic data by extracting the local database schema and passing it to an LLM (local via Ollama/LM Studio, or cloud via OpenAI API key). The developer runs `devx db synthesize postgres --records 100` and gets chaotic INSERT statements (weird Unicode, extreme lengths, missing fields) piped directly into their container — catching bugs that "perfect" seed data misses.

* **Sibling Implementations Studied:**
  * [cmd/db_seed.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/db_seed.go) — **Closest sibling.** Uses a per-command `--runtime` string flag (`dbSeedRuntime`, line 41), resolves engine via `database.Registry[engineName]`, inspects container for mapped host port via `exec.Command(runtime, "inspect", ...)` (line 114), and pipes SQL into the container via `exec.Command("sh", "-c", seedCommand)` (line 210). Uses `NonInteractive` for confirmation bypass (line 180), `DryRun` for safe preview (line 160/165), `outputJSON` for structured output (line 149). Includes gcloud auth recovery loop (line 236).
  * [cmd/db_spawn.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/db_spawn.go) — Shows the container lifecycle pattern: `exec.Command(runtime, "inspect", ...)` for status check (line 90), port conflict handling with interactive retry (line 127-141).
  * [internal/database/engine.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/database/engine.go) — `Engine` struct (line 28): `Name`, `Image`, `DefaultPort`, `InternalPort`, `VolumePath`, `Env`, `ReadyLog`, `ConnStringFmt`. `Registry` map (line 40) has `postgres`, `redis`, `mysql`, `mongo`. `ConnString(port int) string` (line 96) returns formatted connection URL. `SupportedEngines()` (line 120) returns `["postgres", "redis", "mysql", "mongo"]`.
  * [internal/database/snapshot.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/database/snapshot.go) — Uses `provider.ContainerRuntime` (line 66: `CreateSnapshot(rt provider.ContainerRuntime, engine, snapshotName string)`). This is the provider-abstracted pattern, **but** `db_seed.go` and `db_spawn.go` use the simpler `--runtime` string pattern. Our command will follow the `db_seed.go` pattern for consistency with the `db *` command family.
  * [internal/ai/bridge.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ai/bridge.go) — `BridgeEnv` struct (line 31): `EngineName`, `Active`, `EnvVars`. `DiscoverHostLLMs(runtime string) BridgeEnv` (line 40) probes ports 11434 (Ollama) and 1234 (LM Studio). Returns `EnvVars` map with `OPENAI_API_BASE`, `OPENAI_API_KEY`, etc. **Critical:** consumed by `shell.go:168` to inject env vars into containers — must NOT be modified to add cloud API keys.
  * [cmd/shell.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/shell.go#L166-L183) — Only caller of `DiscoverHostLLMs()`. Iterates `aiEnv.EnvVars` and injects them into the container as `-e` flags (line 170-176).
  * [internal/devxerr/error.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/devxerr/error.go) — Highest exit code: `85` (State Replication). New codes start at `86`.
  * [cmd/root.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/root.go#L117-L126) — Global flags: `--env-file`, `--json`, `-y`/`--non-interactive`, `--dry-run`, `--detailed`. Note: `--runtime` is **NOT** a global flag — it is declared per-command (e.g., `db_seed.go:58`).

* **Gap Analysis:**
  1. **`DiscoverHostLLMs()` is container-injection scoped.** It returns env vars designed for container `-e` injection (`host.containers.internal` hostnames). For `db synthesize`, we need the raw localhost URL to make HTTP calls from the host. We must create a **new function** to avoid breaking `shell.go`.
  2. **No existing HTTP client for LLM APIs.** The `internal/ai/` package only has port probing. We need a new `GenerateCompletion()` function.
  3. **Cloud LLM support requires new env var detection.** The user requested support for developers using gemini-cli, claude code, codex, or opencode — who may have `OPENAI_API_KEY`, `GEMINI_API_KEY`, or `ANTHROPIC_API_KEY` already exported. This is separate from local LLM discovery.
  4. **`db_seed.go` pattern uses raw `exec.Command(runtime, ...)`** not `provider.ContainerRuntime`. Since `db synthesize` is a `db *` subcommand, it should follow the same pattern for consistency.

## 2. Design Decisions

### Design Principles Alignment

* [x] **One CLI, everything** — Extends the `devx db` command hierarchy. `synthesize` is a natural companion to `seed` (seed runs your scripts, synthesize uses AI to generate data).
* [x] **Convention over configuration** — Zero-config happy path: if Ollama is running, it just works. If not, checks for `OPENAI_API_KEY` env var. No YAML config required.
* [x] **Transparency** — Before inserting data, prints: schema extracted, number of tables found, target record count, and which LLM provider will be used. Requires confirmation unless `-y`.
* [x] **Idempotency** — INSERTs are additive. Running twice adds more rows. Not destructive (no DROP/TRUNCATE). Safe to re-run.
* [x] **AI-native** — Supports `--json` (returns `{"engine": "...", "tables": N, "records_requested": N, "records_inserted": N, "provider": "..."}`), `--dry-run` (extracts schema + builds prompt, exits before LLM call), deterministic exit codes.
* [x] **CLI + YAML parity** — No new YAML keys needed. LLM discovery is automatic. `--model` CLI flag allows override.
* [x] **Optimized Inner Loop** — Generates realistic test data in seconds vs. hours of manual fixture writing.
* [ ] **Client-Driven Architecture** — N/A (local database operation).
* [x] **Absolute Portability** — Schema extraction via `pg_dump`/`mysqldump` inside the container. LLM API call is pure Go `net/http`. Works on macOS/Linux, Intel/ARM.

### Non-Obvious Choices

1. **Why a new `DiscoverAIProvider()` function instead of modifying `DiscoverHostLLMs()`?** The existing function returns `EnvVars` with container-to-host hostnames (`host.containers.internal`) that `shell.go` injects into containers. If we add cloud API keys to this map, `devx shell` would leak real `OPENAI_API_KEY`/`GEMINI_API_KEY` into every container — a security regression. A separate function avoids this.

2. **Why OpenAI-compatible API only (not native Gemini/Anthropic)?** OpenAI, Gemini, and Anthropic have three fundamentally different JSON schemas. OpenAI-compatible endpoints cover: Ollama, LM Studio, OpenAI Cloud, and any proxy (LiteLLM, etc.). Users with Gemini/Anthropic keys can use a local proxy. This gives us 95% coverage with 33% of the implementation cost.

3. **Why buffer the full LLM response instead of streaming?** LLMs frequently wrap SQL in markdown code blocks (`` ```sql ``). Streaming raw bytes directly into `psql` would cause syntax errors on the backticks. Buffering lets us strip markdown wrappers before piping. A 1000-record INSERT is typically <2MB — trivial to hold in memory.

4. **Why follow the `db_seed.go` pattern (raw `--runtime` string) instead of `snapshot.go` pattern (`provider.ContainerRuntime`)?** All `db *` commands (`spawn`, `seed`, `list`, `rm`, `restart`) use the raw `--runtime` string flag pattern. Using `getFullProvider()` would be inconsistent with the command family. We follow the established pattern and note the inconsistency as existing tech debt.

5. **Why wrap inserts in a transaction?** If the LLM generates 100 INSERTs and #47 fails (e.g., FK violation), we want to roll back cleanly rather than leave the database in a half-populated state. Wrapping in `BEGIN; ... COMMIT;` makes the operation atomic.

## 3. Proposed Changes

### `internal/ai/` — AI Provider Discovery & Completion

* **`[NEW]` `internal/ai/completion.go`:**
  * `AIProvider` struct: `Name string` (e.g., "Ollama", "OpenAI"), `BaseURL string`, `APIKey string`, `DefaultModel string`, `Source string` ("local" or "cloud").
  * `DiscoverAIProvider() (*AIProvider, error)`: Priority cascade:
    1. Probe localhost:11434 (Ollama) → `AIProvider{Name: "Ollama", BaseURL: "http://127.0.0.1:11434/v1", APIKey: "devx-local-ai", DefaultModel: "", Source: "local"}`
    2. Probe localhost:1234 (LM Studio) → similar
    3. Check `os.Getenv("OPENAI_API_KEY")` → `AIProvider{Name: "OpenAI", BaseURL: "https://api.openai.com/v1", APIKey: key, DefaultModel: "gpt-4o", Source: "cloud"}`
    4. Returns `nil, error` if nothing found.
  * `GenerateCompletion(provider *AIProvider, model, systemPrompt, userPrompt string) (string, error)`: POST to `{BaseURL}/chat/completions` with `Authorization: Bearer {APIKey}`. Reads `choices[0].message.content` from response JSON. Timeout: 120s. Returns the raw completion text.

* **`[NEW]` `internal/ai/completion_test.go`:**
  * Mock HTTP server tests for `GenerateCompletion()`: valid response, HTTP 429 rate limit, malformed JSON, timeout.
  * Unit test for `DiscoverAIProvider()` env var fallback (set `OPENAI_API_KEY`, verify it's picked up when no local LLM ports are open).

### `internal/database/` — Schema Extraction & SQL Execution

* **`[NEW]` `internal/database/synthesizer.go`:**
  * `SynthesizableEngines() []string`: Returns `["postgres", "mysql"]`.
  * `ExtractSchema(runtime, engineName string) (string, error)`:
    * `postgres`: `exec.Command(runtime, "exec", "-i", "devx-db-postgres", "pg_dump", "-s", "-U", "devx", "devx")`
    * `mysql`: `exec.Command(runtime, "exec", "-i", "devx-db-mysql", "mysqldump", "--no-data", "-u", "devx", "-pdevx", "devx")`
    * Returns the DDL string or error if container not running.
  * `SanitizeLLMSQL(raw string) string`: Strips `` ```sql ``, `` ``` ``, and leading/trailing whitespace. If result doesn't start with `BEGIN`, wraps in `BEGIN;\n{sql}\nCOMMIT;`.
  * `PipeSQL(runtime, engineName, sql string) error`:
    * `postgres`: `exec.Command(runtime, "exec", "-i", "devx-db-postgres", "psql", "-U", "devx", "-d", "devx")` with `cmd.Stdin = strings.NewReader(sql)`
    * `mysql`: `exec.Command(runtime, "exec", "-i", "devx-db-mysql", "mysql", "-u", "devx", "-pdevx", "devx")` with `cmd.Stdin = strings.NewReader(sql)`

* **`[NEW]` `internal/database/synthesizer_test.go`:**
  * `TestSanitizeLLMSQL`: markdown-wrapped input, raw SQL input, empty input, multiple code blocks, non-SQL markdown.
  * `TestSynthesizableEngines`: verify returns exactly `["postgres", "mysql"]`.

### `internal/devxerr/` — Error Codes

* **`[MODIFY]` [error.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/devxerr/error.go#L53-L59):**
  * Add after line 59:
    ```go
    // Synthetic Data Generation Errors (Idea 57)
    CodeDBSynthNoAI        = 86 // No local LLM or cloud API key found
    CodeDBSynthLLMFailed   = 87 // LLM API request failed or timed out
    CodeDBSynthUnsupported = 88 // Engine does not support DDL extraction (mongo/redis)
    CodeDBSynthSQLFailed   = 89 // Generated SQL failed to execute
    ```

### `cmd/` — CLI Layer

* **`[NEW]` `cmd/db_synthesize.go`:**
  * `Use: "synthesize <engine>"`, `Short: "Generate realistic synthetic data using AI"`.
  * `Long`: Explains schema extraction, LLM integration (local + cloud), supported engines.
  * `Example`: `devx db synthesize postgres`, `devx db synthesize mysql --records 50 --model llama3`.
  * Local flags:
    * `--records` (int, default: 100)
    * `--model` (string, default: "" — uses provider's default)
    * `--runtime` (string, default: "podman" — matches `db_seed.go` pattern)
  * Flow:
    1. Validate engine is in `SynthesizableEngines()`. If not, fail with `CodeDBSynthUnsupported` and message listing supported engines.
    2. Verify container is running: `exec.Command(runtime, "inspect", containerName, "--format", "{{.State.Running}}")`.
    3. Call `ai.DiscoverAIProvider()`. If nil, fail with `CodeDBSynthNoAI` and actionable message: "No AI provider found. Start Ollama (`ollama serve`) or export OPENAI_API_KEY."
    4. Call `database.ExtractSchema(runtime, engineName)`.
    5. Build system prompt (see below). Build user prompt with schema + record count.
    6. **`--dry-run` gate**: Print extracted schema, built prompt, discovered provider. Exit before LLM call.
    7. Interactive confirmation (unless `-y`): "About to generate {N} synthetic records for {engine} via {provider}. Continue?"
    8. Call `ai.GenerateCompletion()`. Print inline progress: `"🤖 Generating {N} records via {provider}..."`.
    9. Call `database.SanitizeLLMSQL()`.
    10. Call `database.PipeSQL()`.
    11. Print success or `--json` output.
  * System prompt (hardcoded):
    ```
    You are an expert database administrator. Generate realistic, chaotic synthetic data as raw SQL INSERT statements. Rules:
    1. Output ONLY valid SQL. No markdown, no explanations, no code blocks.
    2. Include edge cases: Unicode characters (Japanese, Arabic, emoji), very long strings (200+ chars), NULL values where columns are nullable, minimum and maximum numeric values, dates spanning decades.
    3. Respect foreign key relationships: insert parent rows before child rows.
    4. Wrap all statements in BEGIN; ... COMMIT;
    ```

### Architecture & Runtime Compliance Checklist

* [x] **Provider Abstraction:** Uses per-command `--runtime` string flag, matching `db_seed.go`/`db_spawn.go` pattern. Does NOT use `getFullProvider()` — consistent with `db *` command family.
* [x] **Config Cascade:** No new YAML keys. LLM discovery is automatic (local probe → env var fallback).
* [x] **Global Flags:** `--dry-run` exits before LLM call (no network I/O). `-y` bypasses confirmation. `--json` returns structured output. `--runtime` is a local flag (not global).
* [x] **Error Handling Strategy:** Fail-fast with deterministic exit codes. Unsupported engine → 88. No AI → 86. LLM failure → 87. SQL execution failure → 89. All errors include actionable remediation messages.
* [x] **Environment & Context:** All commands execute on the **host**. Schema extraction shells into the container via `exec.Command(runtime, "exec", ...)`. LLM API call is `net/http` from the host. SQL piping is `exec.Command(runtime, "exec", "-i", ...)` with stdin.
* [x] **Pre-push Hook / `devx audit`:** No impact. This feature does not change exit codes, container execution paths, or scanning behavior.

## 4. Configuration & Schema Changes

* [ ] **`devx.yaml` keys:** None. This feature is zero-config by design.
* [ ] **Environment variables:** Reads (does not set): `OPENAI_API_KEY`, `OPENAI_API_BASE` (for custom endpoints). No new `DEVX_*` env vars.
* [ ] **CLI flags:**
  * `devx db synthesize --records <int>` (default: 100)
  * `devx db synthesize --model <string>` (default: "" = provider default)
  * `devx db synthesize --runtime <string>` (default: "podman")

## 5. Documentation Ecosystem (Mandatory)

### Official Docs & CLI
* [ ] **Official Docs (VitePress):** Add "AI Synthetic Data" section to [docs/guide/databases.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/databases.md) after the `db snapshot` section (line 178). No new page needed — it fits naturally in the database guide.
* [ ] **CLI Help Text:** `Use`, `Short`, `Long`, `Example` in `cmd/db_synthesize.go`. Registered under `dbCmd` (inherits `GroupID: "infra"`).
* [ ] **Environment Health (`devx doctor`):** Add entry to [cmd/doctor.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/doctor.go#L251-L297) `computeFeatureReadiness()`:
  ```go
  {
      command: "devx db synthesize",
      ready:   (tools["podman"] || tools["docker"]) && (isPortOpen("11434") || os.Getenv("OPENAI_API_KEY") != ""),
      missing: "Ollama or OPENAI_API_KEY",
  },
  ```
* [ ] **Main README:** Add `devx db synthesize` to Database capabilities section.
* [ ] **Feature Trackers:** Migrate Idea 57 from `IDEAS.md` to `FEATURES.md`.
* [ ] **CHANGELOG:** Auto-generated by release-please. Commit: `feat(db): add AI-driven synthetic data generation (Idea 57)`.

### Agent Skills Templates (Mandatory Audit)

> [!CAUTION]
> All 4 files must be updated with the new `devx db synthesize` command, flags, and exit codes 86-89.

* [ ] `.agents/skills/devx/SKILL.md`
* [ ] `.agents/skills/platform-engineer/SKILL.md`
* [ ] `.github/skills/devx/SKILL.md`
* [ ] `.github/skills/platform-engineer/SKILL.md`

### Example Configs & Templates
* [ ] **`devx.yaml.example`:** No changes needed (zero-config feature).
* [ ] **`CONTRIBUTING.md`:** Add `internal/ai/completion.go` to the project structure diagram (line 70 area, after `│   ├── ai/`).

## 6. Verification Plan

### Automated Tests
* [ ] `internal/database/synthesizer_test.go` — `SanitizeLLMSQL()` round-trips, `SynthesizableEngines()`.
* [ ] `internal/ai/completion_test.go` — Mock HTTP server for `GenerateCompletion()`, env var fallback for `DiscoverAIProvider()`.
* [ ] `go vet ./...`
* [ ] `golangci-lint run ./...` (errcheck + staticcheck)
* [ ] `mage licensecheck`
* [ ] `go build ./...`
* [ ] All 7 GitHub Actions checks pass (Lint, Test, Validate Butane, Build ×4)

### Manual / Edge Case Verification
* [ ] **Edge Case 1:** DB container not running → *Expected: `"the local postgres database is not running — start it with 'devx db spawn postgres'"`*
* [ ] **Edge Case 2:** Unsupported engine (`redis`, `mongo`) → *Expected: fail-fast with `CodeDBSynthUnsupported` (88) listing supported engines*
* [ ] **Edge Case 3:** No Ollama running AND no `OPENAI_API_KEY` → *Expected: `CodeDBSynthNoAI` (86) with actionable message*
* [ ] **Edge Case 4:** LLM returns markdown-wrapped SQL → *Expected: `SanitizeLLMSQL` strips wrappers, SQL executes cleanly*
* [ ] **Edge Case 5:** LLM returns malformed SQL → *Expected: `CodeDBSynthSQLFailed` (89), prints the raw SQL and the DB error for debugging*
* [ ] **Edge Case 6:** `--dry-run` → *Expected: prints schema, prompt, provider info. No LLM call, no data inserted*
* [ ] **Edge Case 7:** `--json` → *Expected: valid JSON with `engine`, `tables`, `records_requested`, `provider` fields*
* [ ] **Edge Case 8:** Empty schema (no tables) → *Expected: "No tables found in postgres. Run your migrations first."*
* [ ] **E2E smoke test:** `devx db spawn postgres` → `devx db seed postgres` → `devx db synthesize postgres --records 10` → verify rows via `psql`.

## 7. Self-Review Checklist

- [x] I have **read every file I plan to modify** — `internal/ai/bridge.go` (92 lines), `internal/devxerr/error.go` (92 lines), `cmd/doctor.go` (528 lines, `computeFeatureReadiness()` at L251-L297).
- [x] I have **cross-referenced every function call** — `database.Registry[engineName]` at `engine.go:40`, `database.SupportedEngines()` at `engine.go:120`, `ai.DiscoverHostLLMs(runtime)` at `bridge.go:40` (will NOT modify, creating new function instead), `isPortOpen(port)` at `bridge.go:80`.
- [x] I have **not omitted documentation updates** — VitePress (databases.md), README, doctor, CHANGELOG, all 4 agent skill files, CONTRIBUTING.md all listed.
- [x] My verification plan **tests edge cases** — 8 edge cases covering container down, unsupported engine, no AI, markdown wrapping, malformed SQL, dry-run, JSON, empty schema.
- [x] I have **not assumed default values** — Verified `--runtime` defaults to "podman" (matching `db_seed.go:58`), global flags in `root.go:117-126`.
- [x] I have audited all **4 agent skill template files** — `.agents/skills/devx/SKILL.md`, `.agents/skills/platform-engineer/SKILL.md`, `.github/skills/devx/SKILL.md`, `.github/skills/platform-engineer/SKILL.md`.
- [x] I have verified that the project/task status (**IDEAS.md → FEATURES.md**) will be updated.

---

## 8. Open Questions / User Review Required

> [!WARNING]
> **Streaming vs. Buffering:** We will buffer the full LLM response, strip markdown wrappers, then pipe sanitized SQL into the database. A 1000-record INSERT is typically <2MB. This ensures reliability. **Approved?**

> [!IMPORTANT]
> **Cloud API scope:** We will support OpenAI-compatible API only (covers Ollama, LM Studio, OpenAI Cloud, and any proxy). Gemini and Anthropic native APIs have completely different JSON schemas and are not worth the implementation cost when users can use `OPENAI_API_BASE` pointed at a proxy. **Approved?**
