# Audit: Gemini's Implementation Plan for Idea 57

**Auditor:** Claude  
**Date:** 2026-05-03  
**Scope:** Template compliance + Technical correctness  

---

## Summary Verdict

Gemini's plan is a **skeleton**, not a plan. It covers the "what" at a high level but omits almost every structural section required by the [template](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/plans/template_implementation_plan.md), and several of its technical claims are either incorrect or unverified against the live codebase. For comparison, see how Idea 56 was planned in [implementation_plan_idea_56.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/plans/implementation_plan_idea_56.md) — that plan has 236 lines with 8 edge cases, a self-review checklist, full function signature verification, and every documentation file enumerated.

Gemini's plan is **94 lines** and is missing **7 of the 8 mandatory template sections**.

---

## Template Compliance (Section-by-Section)

| Template Section | Present? | Notes |
|---|---|---|
| **§1 Deep Research & Context** | ❌ Missing | No `Objective` statement. No `Sibling Implementations` studied. No `Gap Analysis`. The plan doesn't mention reading `db_seed.go`, `db_spawn.go`, `snapshot.go`, or `shell.go` — the four most relevant sibling files. |
| **§2 Design Decisions** | ❌ Missing | No design principles checklist. No `Non-Obvious Choices` section. The streaming vs buffering decision is called out but placed under "User Review Required" instead of the proper section. |
| **§3 Proposed Changes** | ✅ Partial | Files are listed but descriptions lack specificity (see correctness issues below). |
| **Architecture & Runtime Compliance Checklist** | ❌ Missing | This is the most critical omission. No provider abstraction analysis. No global flags audit. No error handling strategy. No environment/context specification. |
| **§4 Configuration & Schema Changes** | ❌ Missing | No YAML keys, env vars, or CLI flags are formally specified. The `--model` and `--records` flags are mentioned in passing but not in a dedicated section. |
| **§5 Documentation Ecosystem** | ❌ Inadequate | Lists 4 items but the template requires auditing **all 4 agent skill files**, `devx.yaml.example`, `CONTRIBUTING.md`, `devx doctor`, CLI help text, and VitePress sidebar wiring. Gemini lists only 1 of the 4 skill files. |
| **§6 Verification Plan** | ❌ Inadequate | Only 2 unit tests and 6 manual steps. No edge cases enumerated. No `go vet`, `staticcheck`, `mage licensecheck`, or CI check requirements. Compare: Idea 56 has 8 explicit edge cases with expected behaviors. |
| **§7 Self-Review Checklist** | ❌ Missing | Not present at all. |

---

## Technical Correctness Issues

### GAP 1: `ExtractSchema()` uses raw `exec.Command(runtime, ...)` — violates provider abstraction

> [!CAUTION]
> Gemini's plan says `ExtractSchema` will execute `podman exec -i devx-db-postgres pg_dump ...`. But this hardcodes the runtime string.

**The codebase has TWO runtime patterns:**
1. **`db_seed.go` / `db_spawn.go` pattern:** Uses a local `--runtime` string flag and calls `exec.Command(runtime, ...)` directly. This is the older pattern used by simpler db commands.
2. **`snapshot.go` / `state_checkpoint.go` pattern:** Uses `getFullProvider()` → `prov.Runtime` → `rt.CommandContext()`. This is the correct provider-abstracted pattern.

Gemini's plan says `ExtractSchema(runtime string, engine Engine)` takes a raw `runtime` string. **This follows the older pattern (1), not the preferred pattern (2).** Since `db synthesize` is a new command interacting with containers, it should use `provider.ContainerRuntime` — the same way `snapshot.go:CreateSnapshot(rt provider.ContainerRuntime, ...)` does.

**Fix:** `ExtractSchema` should accept `rt provider.ContainerRuntime` instead of `runtime string`.

### GAP 2: `ExecuteSQL()` — same provider abstraction violation

Same issue. `ExecuteSQL(runtime string, engine Engine, sql string)` should use `rt provider.ContainerRuntime` and call `rt.CommandContext()` to pipe SQL, not `exec.Command(runtime, "exec", ...)`.

### GAP 3: `DiscoverHostLLMs()` modification — breaks existing caller

> [!WARNING]
> Gemini plans to "enhance `DiscoverHostLLMs(runtime)` to act as a general AI capability detector" that falls back to checking `OPENAI_API_KEY`, `GEMINI_API_KEY`, `ANTHROPIC_API_KEY` environment variables.

This function currently returns a `BridgeEnv` whose `EnvVars` map is injected **as container environment variables** by `shell.go:170`. If we add cloud API keys to the `EnvVars` map, then `devx shell` will start injecting the user's **real cloud API keys** into every container as environment variables. That's a security concern and a behavioral change to an existing feature.

**Fix:** Either:
- Create a **separate** function `DiscoverAIProvider(runtime string) BridgeEnv` that handles both local and cloud, and use it only from `db_synthesize.go`.
- Or add a `Source` field to `BridgeEnv` (e.g., `"local"` vs `"cloud"`) so `shell.go` can filter appropriately.

### GAP 4: Cloud API endpoint differences are hand-waved

The plan says `GenerateCompletion` will "inspect the BridgeEnv to determine the target API (OpenAI-compatible local, OpenAI Cloud, Google Gemini, or Anthropic) and format the JSON payload accordingly." But:
- **OpenAI:** `POST https://api.openai.com/v1/chat/completions` with `Authorization: Bearer $OPENAI_API_KEY`
- **Gemini:** `POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key=$GEMINI_API_KEY` — completely different JSON schema (uses `contents[].parts[]` not `messages[]`)
- **Anthropic:** `POST https://api.anthropic.com/v1/messages` with `x-api-key` header and `anthropic-version` header — yet another different schema

These are three fundamentally different APIs. The plan doesn't acknowledge this complexity at all. This is non-trivial to implement correctly and warrants explicit design decisions about whether to implement all three or just the OpenAI-compatible subset (which covers Ollama, LM Studio, and OpenAI Cloud).

**Recommendation:** Support OpenAI-compatible API only (covers local LLMs + OpenAI Cloud + any proxy). For Gemini/Anthropic, note that users can use a local proxy like LiteLLM or just export `OPENAI_API_BASE` pointed at their preferred provider's OpenAI-compatible endpoint.

### GAP 5: `--runtime` flag doesn't exist as a global flag

Gemini's plan lists `--runtime` under "Global Flags Handled." But `--runtime` is **NOT a global flag**. Looking at [root.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/root.go#L117-L126), the global flags are: `--env-file`, `--json`, `-y`/`--non-interactive`, `--dry-run`, `--detailed`.

Runtime resolution is done via `getFullProvider()` in the provider-abstracted commands, or via a per-command `--runtime` string flag (like `db_seed.go:58` declares `dbSeedRuntime`). Gemini needs to either:
- Use `getFullProvider()` (preferred), or
- Declare a local `--runtime` flag on `db_synthesize` (matches `db_seed.go` pattern)

### GAP 6: No `-y` / `--non-interactive` handling specified

The plan doesn't mention how `-y` is handled. `db_seed.go` uses `NonInteractive` to bypass the confirmation prompt. `db synthesize` should do the same — ask "About to insert N synthetic records into postgres. Continue?" unless `-y`.

### GAP 7: Prompt engineering is underspecified

The system prompt shown is a single sentence. For reliable SQL generation, you need:
- Explicit instructions to NOT wrap in markdown
- Table-by-table generation hints
- Foreign key awareness (insert parent rows before child rows)
- Transaction wrapping (`BEGIN; ... COMMIT;`)
- Explicit instruction to use the exact column names from the schema

### GAP 8: `BridgeEnv` struct needs new fields for cloud support

Currently `BridgeEnv` has `EngineName`, `Active`, and `EnvVars`. For cloud providers, we need:
- `APIBase` (the endpoint URL)
- `APIKey` (the auth credential)
- `Source` (`"local"` or `"cloud"`)
- `DefaultModel` (provider-specific default)

The plan doesn't specify these struct changes.

---

## Missing Documentation Items

Per the template, these are required but Gemini omitted them:

| Item | Status |
|---|---|
| `.agents/skills/devx/SKILL.md` | ✅ Listed |
| `.agents/skills/platform-engineer/SKILL.md` | ❌ Missing |
| `.github/skills/devx/SKILL.md` | ❌ Missing |
| `.github/skills/platform-engineer/SKILL.md` | ❌ Missing |
| `devx.yaml.example` | ❌ Missing (no new YAML keys, but `ai:` config may need updating) |
| `CONTRIBUTING.md` | ❌ Missing |
| `devx doctor` | ❌ Missing (should check: Ollama/LM Studio running OR cloud API key present) |
| VitePress sidebar wiring (`config.mjs`) | ❌ Missing |
| CLI Help Text (`Use`, `Short`, `Long`, `Example`) | ❌ Missing |

---

## Missing Edge Cases (Template §6 requires these)

1. **What if the database container isn't running?** → Expected: fail-fast with actionable error.
2. **What if the schema is empty (no tables)?** → Expected: fail-fast, "No tables found."
3. **What if the LLM returns malformed SQL?** → Expected: show the raw response and the SQL error, suggest `--records` reduction.
4. **What if `--records` is very large (10000)?** → Expected: warn about LLM context window limits.
5. **What if `--dry-run` is passed?** → Expected: extract schema, print prompt, exit before LLM call.
6. **What if `--json` is passed?** → Expected: valid JSON output with fields.
7. **What if the foreign key ordering causes constraint violations?** → Expected: documented limitation or topological sort.
8. **What if the user runs against a non-`devx` managed DB?** → Expected: container name pattern mismatch, clear error.

---

## Recommendations

1. **Rewrite the plan using the template sections verbatim.** Copy the template and fill in each section.
2. **Read the sibling files first.** `db_seed.go` is the closest pattern. `snapshot.go` shows the provider-abstracted pattern.
3. **Use `provider.ContainerRuntime`** for `ExtractSchema` and `ExecuteSQL`, not raw runtime strings.
4. **Don't modify `DiscoverHostLLMs()`** — create a new `DiscoverAIProvider()` function to avoid breaking `devx shell`.
5. **Scope cloud support to OpenAI-compatible API only.** This covers Ollama, LM Studio, OpenAI, and any proxy. Don't try to implement Gemini and Anthropic native APIs — the ROI is poor when users can just use a proxy.
6. **Add the self-review checklist** with evidence that every file was read.
