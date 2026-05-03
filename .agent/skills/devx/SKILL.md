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

## 📁 6. Configuration Discovery (Upward Traversal)

`devx` and `devx cluster` commands automatically traverse upward from the current working directory to discover their configuration files (`devx.yaml` and `cluster.yaml`, respectively). 
You do NOT need to `cd` back to the repository root to run `devx` commands. You can safely execute them from deep within nested subdirectories.

## 🔀 7. Advanced Orchestration (`devx up`)

You do not need to manually boot components sequentially and wait for them. `devx` features a robust DAG (Directed Acyclic Graph) orchestrator.
Running `devx up` will automatically spawn all mapped databases, native services, and network tunnels in parallel, natively respecting `depends_on` wait conditions.
If you or the user require a different topological slice of the system (e.g. bypassing the frontend React app to work only on the APIs), you can apply additive overlays to the execution via flags like `devx up --profile backend-only`.

## 🌐 8. Dynamic Port Shifting & Discovery

`devx` automatically negotiates and shifts ports if collisions (like a ghost `node` process on `:8080`) occur. Do NOT blindly assume services or databases map statically to their default host ports.
To discover where a service or database is actually running, ALWAYS query the machine-readable state via `devx db list --json` and `devx tunnel list --json`. When writing scripts, always rely on the dynamically injected `.env` variables (e.g., `$PORT`, `$DATABASE_URL`) rather than hardcoding port strings.

## 📚 9. Documentation is Mandatory (Definition of Done)

When shipping features on the `devx` CLI, the task is **not done** until the official documentation has been updated. Missing or outdated documentation directly harms the Developer Experience (DevX) mission.

- **Checklist Requirement:** Every implementation plan (`implementation_plan.md`) and task tracker (`task.md`) you create MUST include a mandatory phase: `Documentation Updating`.
- **Validation:** Before running the `/push` workflow to cut a release, PR, or commit, you must pause and explicitly review the `docs/guide/` directory to ensure all new commands, flags, and `devx.yaml` schema fields are documented.
- **Example configs:** Schema changes MUST be reflected with thorough examples in `devx.yaml.example`.
- **Taxonomy Synchronization:** If you restructure the VitePress sidebar (`docs/.vitepress/config.mjs`), you MUST synchronously replicate the exact same category structure across:
  1. `README.md` (in the "Why devx?" feature groups)
  2. `cmd/root.go` (in the `Feature Ecosystem` help text and `rootCmd.AddGroup` definitions)
  3. Relevant `cmd/*.go` files (updating the `GroupID` assignments)
  These four locations form the single source of truth for the project's capabilities and must never drift.

## 🛠️ 10. CI and Task Execution (`devx action`)

Do NOT manually run native build or test commands (like `npm run build`, `go test`, `cargo test`, `pytest`, etc.) yourself.

Any time you need to execute unit tests, integration tests, or verify compilation, use `devx`'s custom action runner by invoking predefined sequences defined in the project's `devx.yaml`:

- `devx action ci`: Executes the standard CI pipeline for this project.
- `devx action test`: Executes the standalone test suite (if defined).
- `devx action <action-name> --detailed`: Use this if a step fails and you need the full verbose terminal output (instead of the concise TUI summary) to debug the failure log.

These devx wrappers natively emit rich telemetry for Grafana observability and enforce deterministic test environments. This rule is absolute: **Dogfood the local CLI.**

## 🚀 11. Ship Code with `devx agent ship` or `review` (MANDATORY)

You are **FORBIDDEN** from using raw `git push` in this repository. A pre-push git hook will block you if you try. Instead, you MUST use `devx agent` commands to commit, push, and verify CI.

**For automated merging (Default):**
```bash
devx agent ship -m "feat: your commit message"
```

This command orchestrates the full lifecycle:
1. **Pre-flight:** Runs local tests, lint, and build automatically
2. **Commit & Push:** Stages, commits, pushes (it bypasses the hook internally)
3. **PR & CI Poll:** Creates a PR and blocks your terminal until the CI pipeline completes
4. **Merge:** Squash-merges the PR only if CI is green

**For human review (No Auto-Merge):**
If the human specifically requests you to create a PR for them to review, use:
```bash
devx agent review -m "feat: your commit message"
```
This does the exact same pre-flight, push, and blocking CI poll as `ship`, but it **leaves the PR open** for human review instead of merging it.

If the pipeline fails, both commands return Exit Code `53` with condensed failure logs. You MUST fix the issue and re-run the command immediately.

For machine-readable output: `devx agent ship -m "message" --json`

## 🔗 12. Hybrid Bridge (`devx bridge`)

Connect the local environment to remote Kubernetes services. Bridge follows the **Client-Driven Architecture** principle — no permanent cluster-side controllers.

### Commands

**Outbound (Idea 46.1):**
- `devx bridge connect --json`: Establish outbound bridge to remote cluster services
- `devx bridge status --json`: Show active bridge and intercept sessions
- `devx bridge disconnect -y`: Tear down all active bridges and intercepts

**Inbound (Idea 46.2):**
- `devx bridge intercept <service> --steal --json`: Intercept inbound cluster traffic to local
- `devx bridge intercept <service> --steal --dry-run`: Preview without modifying cluster
- `devx bridge rbac`: Generate minimum-privilege RBAC manifest for intercept
- `devx bridge rbac -n staging`: Namespace-scoped RBAC

**Hybrid Topology (Idea 46.3) — `runtime: bridge` in `devx up`:**
Bridge services declared inline in `devx.yaml` with `runtime: bridge` participate in the `devx up` DAG orchestrator. Two sub-types:
- `bridge_target`: Outbound port-forward to a remote K8s service
- `bridge_intercept`: Inbound traffic steal from a remote K8s service to local

DAG-managed sessions are tagged `Origin: "dag"` — `devx bridge disconnect` skips them.
`bridge.env` is auto-generated after bridge nodes are healthy.

### Connect CLI Flags
- `--kubeconfig`: Override kubeconfig path
- `--context`: Override kube context
- `-n, --namespace`: Default namespace for targets
- `-t, --target`: Ad-hoc target (repeatable): `service:port` or `service:port:localport`

### Intercept CLI Flags
- `--steal`: Full traffic redirect (required — explicit acknowledgment)
- `--mirror`: Duplicate traffic only (not yet implemented — 46.2b)
- `-p, --port`: Remote port to intercept (default: first port on Service)
- `--local-port`: Local port to route traffic to (default: same as --port)
- `--agent-image`: Override default agent container image (air-gapped clusters)
- `--kubeconfig`, `--context`, `-n`: Same as connect

### State Files
- `~/.devx/bridge.json`: Active session state (port-forwards + intercepts)
- `~/.devx/bridge.env`: Environment variables (auto-injected by `devx shell`)

### Exit Codes
| Code | Meaning |
|------|---------|
| 60 | Kubeconfig not found |
| 61 | Cluster unreachable |
| 62 | Namespace not found |
| 63 | Service not found |
| 64 | Port-forward failed |
| 65 | Agent Job failed to deploy |
| 66 | Agent health check timed out |
| 67 | Failed to patch Service selector |
| 68 | Insufficient RBAC permissions |
| 69 | Service already intercepted |
| 70 | UDP port (not supported) |
| 71 | Yamux tunnel failed |
| 72 | Service not interceptable (ExternalName / no selector) |

### Environment Variables
Bridge generates these per-service variables in `~/.devx/bridge.env`:
- `BRIDGE_<SERVICE>_URL=http://127.0.0.1:<port>`
- `BRIDGE_<SERVICE>_HOST=127.0.0.1`
- `BRIDGE_<SERVICE>_PORT=<port>`

Service names are normalized: `payments-api` → `BRIDGE_PAYMENTS_API_URL`.

### Intercept Architecture
The agent is a self-healing Kubernetes Job with:
- **Dynamic Pod spec** — mirrors target Service's `containerPorts` (including named ports)
- **Yamux tunnel** — multiplexed over `kubectl port-forward` for bidirectional traffic
- **Dedicated ServiceAccount** — narrow RBAC scoped to `update` on the target Service
- **Self-healing** — on tunnel drop or SIGTERM, agent restores the original selector before exiting
- **activeDeadlineSeconds: 14400** (4h auto-cleanup safety net)

