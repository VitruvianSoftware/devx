# Ephemeral E2E Browser Testing Environments

This plan addresses Idea #30 by implementing the `devx test ui` command. The objective is to allow developers to run sophisticated integration/UI tests (e.g., Cypress, Playwright) locally without polluting their active development databases or encountering port collisions between multiple isolated microservices.

## Proposed Changes

### Configuration Architecture (CLI + YAML Hybrid)
> **Design Principle Addressed**: "Always provide both parameter and yaml based approach for maximum flexibility."
> *(Note: This principle will also be explicitly added to `docs/guide/introduction.md#design-principles` during implementation).*

`devx` will parse `devx.yaml` for a `test:` block, but allow overriding or skipping via CLI flags:

**YAML Pattern:**
```yaml
test:
  ui:
    setup: "npm run prisma db push"   # Pre-processing steps
    command: "npx playwright test"    # The actual test execution
```

**CLI Parameter Alternative:**
```bash
devx test ui --setup "npm run prisma db push" -- "npx playwright test"
```

### CLI Component: `cmd/test_ui.go`
- Sets up the `devx test ui` command and its flags (`--setup`).
- Resolves the execution strategy (merging YAML vs CLI inputs).
- Discovers the databases to provision by reading `devx.yaml` `databases:`.
- Orchestrates via `ephemeral.RunFlow()`.

### Ephemeral Engine: `internal/testing/ephemeral.go`
- **Boot Sequence:** 
  1. Generates a unique short UUID to act as a namespace (e.g., `ab3f9`).
  2. Spawns isolated Podman containers (`devx-db-<engine>-ephemeral-<uuid>`) using dynamically acquired free host ports.
  3. Uses anonymous Docker/Podman volumes to guarantee pristine schema states without clashing against primary developer volumes.
- **Context Injection & Pre-Processing:** 
  - Extracts the resulting `engine.ConnString(randomPort)`.
  - Injects `DATABASE_URL` (and `<ENGINE>_URL`) natively into `os.Environ()`.
  - Executes the pre-processing `setup` steps (e.g., DB migrations) so the environments mimic actual deployments, shifting validation left.
- **Test Execution:**
  - Injects the environment into a new shell executing the primary `command`.
- **Teardown Phase:**
  - Wraps the entire lifecycle in strict `defer` destructors blocking `<Ctrl+C>` and OS interrupts.
  - Ensures the ephemeral containers and attached test volumes are instantly destroyed (`rm -f -v`) the moment the test finishes (irrespective of exit code / crash status).

## Open Questions (Answered)

1. **Do we need to support automated pre-processing steps?**
   *Resolved:* Yes. An idempotent pre-processing hook (`setup`) will be provided in both YAML and CLI. It will replicate app deployment migrations (like Prisma DB pushes) shifting infrastructure checks left.

2. **What do you mean by multiple identical databases?**
   *Clarification:* `devx.yaml` currently restricts architecture mapped explicitly by the underlying `engine` key (e.g. you can't boot two isolated Postgres DBs via `devx db spawn` today, it's just `devx-db-postgres`). 
   *Recommendation:* Since `devx db spawn` currently only supports one database container per engine type, our `ephemeral` engine should map 1-to-1 with this exact behavior (so if you have a Postgres DB, the test clones exactly ONE Postgres ephemeral DB). We do not need complex multi-database differentiation yet.

## Verification Plan

### Agent-Driven Verification & Visual Proof
1. I will programmatically generate a dummy application with a `devx.yaml` containing a Postgres database requirement and an arbitrary test script.
2. I will execute `devx test ui -- npm run wait-for-test` using a terminal tool or pseudo-terminal hook.
3. I will run a concurrent `run_command` check against `devx logs` and `podman ps` to statically capture the isolated `ephemeral-<uuid>` host-port running alongside the primary development database.
4. I will embed this terminal CLI output as "Visual Proof" directly into the `docs/guide/` Vitepress environment, aligning with the Platform Engineer SOP.
