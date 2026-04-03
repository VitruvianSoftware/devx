---
trigger: always_on
---

# Planning Requirements

When creating implementation plans, you MUST follow this structured, mature planning process. Shallow or hand-wavy plans are unacceptable.

## Phase 1: Deep Research (Before Writing the Plan)

Before proposing any changes, you MUST:

1. **Read every file you plan to modify.** Do not guess at APIs, struct signatures, or function parameters. Verify them against the live codebase.
2. **Identify established patterns.** Find at least one existing sibling implementation (e.g., if adding `db_seed.go`, study `db_pull.go` and `db_spawn.go` first). Your implementation must follow the same conventions.
3. **Audit global flag compliance.** Every command in the codebase must honor the project's global flags (e.g., `--dry-run`, `-y`, `--json`, `--runtime`). If the project uses global flags, your plan must explicitly state how each is handled.
4. **Trace runtime dependencies.** If your feature depends on external state (running containers, mapped ports, authenticated sessions), specify exactly how you will resolve that state. Do not hand-wave with vague phrases like "retrieve the dynamically mapped port" — specify the exact mechanism (command, API call, inspect format string).

## Phase 2: Writing the Plan

Your implementation plan MUST include:

### Completeness Checklist
- [ ] **Every file to be modified or created is listed** with specific, concrete changes — not vague bullets.
- [ ] **Edge cases are addressed** (What if the container isn't running? What if the port was auto-bumped? What if auth is expired?).
- [ ] **Error handling strategy is defined** (fail-fast vs. interactive recovery, error message quality).
- [ ] **Environment and context** — specify WHERE commands execute (host vs. container), what env vars are available, and what assumptions are made.

### Mandatory Sections
1. **Design Decisions**: Explicitly call out non-obvious choices and why alternatives were rejected.
2. **Gap Analysis**: Compare your plan against the existing codebase and call out any inconsistencies or latent bugs you found during research.
3. **Documentation Updates**: Every plan MUST include a documentation section listing official docs, feature trackers, README, and changelog files that will be updated AFTER successful verification. Documentation is never optional.
4. **Verification Plan**: Must include:
   - Specific test commands to run (not just "test it")
   - Edge case scenarios to verify
   - Build validation commands (linters, doc builders, etc.)
   - A final step to update official documentation after all verifications pass

## Phase 3: Self-Review (Before Presenting)

Before presenting the plan for approval, perform a self-review:

1. **Re-read the plan as a skeptical reviewer.** Would a senior engineer approve this? Are there vague areas?
2. **Cross-reference every API/function call** against the actual source code. If you reference `RecoverGcloudAuth(err)`, verify the actual function signature accepts that type.
3. **Check for completeness gaps:**
   - Did you forget any global flags?
   - Did you specify the documentation update step?
   - Did you address what happens on failure?
   - Is your verification plan testing edge cases, not just the happy path?

## Anti-Patterns (Never Do These)

- ❌ Proposing changes to files you haven't read
- ❌ Using phrases like "retrieve the port" without specifying HOW
- ❌ Omitting documentation updates from the plan
- ❌ Writing a verification plan that only tests the happy path
- ❌ Assuming default values without checking if they can be overridden at runtime
- ❌ Skipping the self-review phase

## Gemini Added Memories
- When asked about events, always check these calendars in addition to the primary calendar: james.nguyen@flyr.com and platform@abrial.ai
