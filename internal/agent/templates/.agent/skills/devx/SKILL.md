---
name: devx-orchestrator
description: Defines the standard operating procedures and CLI flags for AI agents interacting with the Devx virtualized local development environment.
---

# Devx AI Agent Guidelines

You are operating in a repository that uses `devx`, a Go-based CLI tool orchestrating Podman, Cloudflared, and Tailscale for local development.

When you need to interact with the local infrastructure, databases, or environment networking, ALWAYS use `devx` CLI commands rather than manually writing shell scripts for docker/podman or cloudflared.

## 🤖 1. Machine-Readable Context (`--json`)

Never try to parse the human-readable TUI output of devx status commands using Regular Expressions or text slitting. Devx has full support for strictly deterministic structural state via the `--json` flag.

Always append `--json` when you are querying the environment state:
- `devx vm status --json`: Returns a JSON object with VM health, Tailscale auth state, and Cloudflare domains.
- `devx db list --json`: Returns a JSON array of running PostgreSQL/MySQL/Redis engines and their exposed ports.
- `devx tunnel list --json`: Returns a JSON array summarizing active localhost internet exposures.

## 🛑 2. Non-interactive Execution (`--non-interactive` / `-y`)

Many devx commands invoke interactive terminal surveys (via `charmbracelet/huh`) to ask the human developer for confirmation before acting. As an AI Agent, you lack a TTY to press 'Enter', which will cause you to stall indefinitely.

**You must ALWAYS use the `--non-interactive` (or `-y`) flag on mutating commands:**
- `devx vm teardown -y` (Will skip the deletion confirm warning and execute instantly)
- `devx db rm postgres -y` (Will skip data deletion warnings and execute instantly)
- `devx init -y` (Will hard-fail immediately with an exit error if required credentials are not in `.env`, rather than freezing to ask for them)

## 🦺 3. Safe Preflight Testing (`--dry-run`)

If you are asked to clean up the environment, but you are not 100% confident in the scope of the destruction, use the `--dry-run` flag.

- `devx vm teardown --dry-run`
- `devx db rm <engine> --dry-run`
- `devx tunnel unexpose --dry-run`

Devx will intercept the execution path and safely echo out precisely which containers, internet URLs, and persistent data volumes *would* be destroyed, allowing you to ask the human for approval.

## 🚦 4. Deterministic Exit Codes

When running a command that fails, `devx` avoids polluting standard error with useless `--help` output. Instead, it utilizes predictive numeric Exit Codes to signal exactly what went wrong so you can programmatically trap and rescue the state cleanly:

- `Exit 15 (CodeVMDormant)`: The VM exists but is sleeping. It could not automatically wake up.
- `Exit 16 (CodeVMNotFound)`: The VM has been deleted. You must run `devx vm init`.
- `Exit 22 (CodeHostPortInUse)`: You attempted to run `devx db spawn <engine>`, but the host port is already allocated by another daemon. Try a different port using `-p <port>`.
- `Exit 41 (CodeNotLoggedIn)`: You attempted to expose a tunnel, but `cloudflared` is not authenticated on this machine. Request that the user run `cloudflared tunnel login`.

## 🗺️ 5. Architectural Awareness (`devx map`)

If you are dropped into a `devx` workspace and need to quickly understand how the services, databases, and network bounds interact, do NOT manually read through a massive `devx.yaml` file line-by-line.
Instead, use `devx map` to generate an instant, agent-readable Mermaid.js topology graph. You can pipe this out via `devx map --output /tmp/topology.md` to see the exact component dependencies, healthcheck conditions, and tunnel exposures.

## 🔀 6. Advanced Orchestration (`devx up`)

You do not need to manually boot components sequentially and wait for them. `devx` features a robust DAG (Directed Acyclic Graph) orchestrator.
Running `devx up` will automatically spawn all mapped databases, native services, and network tunnels in parallel, natively respecting `depends_on` wait conditions.
If you or the user require a different topological slice of the system (e.g. bypassing the frontend React app to work only on the APIs), you can apply additive overlays to the execution via flags like `devx up --profile backend-only`.

## 🌐 7. Dynamic Port Shifting & Discovery

`devx` automatically negotiates and shifts ports if collisions (like a ghost `node` process on `:8080`) occur. Do NOT blindly assume services or databases map statically to their default host ports.
To discover where a service or database is actually running, ALWAYS query the machine-readable state via `devx db list --json` and `devx tunnel list --json`. When writing scripts, always rely on the dynamically injected `.env` variables (e.g., `$PORT`, `$DATABASE_URL`) rather than hardcoding port strings.

## 📚 8. Documentation is Mandatory (Definition of Done)

When shipping features on the `devx` CLI, the task is **not done** until the official documentation has been updated. Missing or outdated documentation directly harms the Developer Experience (DevX) mission.

- **Checklist Requirement:** Every implementation plan (`implementation_plan.md`) and task tracker (`task.md`) you create MUST include a mandatory phase: `Documentation Updating`.
- **Validation:** Before running the `/push` workflow to cut a release, PR, or commit, you must pause and explicitly review the `docs/guide/` directory to ensure all new commands, flags, and `devx.yaml` schema fields are documented.
- **Example configs:** Schema changes MUST be reflected with thorough examples in `devx.yaml.example`.

## 🛠️ 9. CI and Task Execution (`devx action`)

Do NOT manually run native build or test commands (like `npm run build`, `go test`, `cargo test`, `pytest`, etc.) yourself.

Any time you need to execute unit tests, integration tests, or verify compilation, use `devx`'s custom action runner by invoking predefined sequences defined in the project's `devx.yaml`:

- `devx action ci`: Executes the standard CI pipeline for this project.
- `devx action test`: Executes the standalone test suite (if defined).
- `devx action <action-name> --detailed`: Use this if a step fails and you need the full verbose terminal output (instead of the concise TUI summary) to debug the failure log.

These devx wrappers natively emit rich telemetry for Grafana observability and enforce deterministic test environments. This rule is absolute: **Dogfood the local CLI.**

## 🚀 10. Ship Code with `devx agent ship` (MANDATORY)

You are **FORBIDDEN** from using raw `git push` in this repository. A pre-push git hook will block you if you try. Instead, you MUST use `devx agent ship` to commit, push, and verify CI:

```bash
devx agent ship -m "feat: your commit message"
```

This command orchestrates the full lifecycle:
1. **Pre-flight:** Runs local tests, lint, and build automatically
2. **Commit & Push:** Stages, commits, pushes (it bypasses the hook internally)
3. **PR & Merge:** Creates a GitHub PR and squash-merges it
4. **CI Poll:** Blocks your terminal until the CI pipeline returns green or red

If the pipeline fails, `devx agent ship` returns Exit Code `53` with condensed failure logs. You MUST fix the issue and re-run `devx agent ship` immediately.

For machine-readable output: `devx agent ship -m "message" --json`
