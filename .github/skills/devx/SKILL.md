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
- `devx vm init -y` (Will hard-fail immediately with an exit error if required credentials are not in `.env`, rather than freezing to ask for them)

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
- `Exit 86 (CodeDBSynthNoAI)`: No local LLM or cloud API key found. Start Ollama (`ollama serve`) or export `OPENAI_API_KEY`.
- `Exit 87 (CodeDBSynthLLMFailed)`: LLM API request failed or timed out during `devx db synthesize`.
- `Exit 88 (CodeDBSynthUnsupported)`: Engine does not support DDL extraction. Only `postgres` and `mysql` are supported.
- `Exit 89 (CodeDBSynthSQLFailed)`: Generated SQL failed to execute against the database container.
- `Exit 90 (CodeDiagnosisTimeout)`: AI diagnosis timed out during failure recovery.
- `Exit 91 (CodeDBAskNoAI)`: No AI provider found for natural language database query.
- `Exit 92 (CodeDBAskQueryFailed)`: Generated SQL failed to execute during `devx db ask`.
- `Exit 93 (CodeDBAskReadOnly)`: Write attempted without `--allow-writes` during `devx db ask`.

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

## 🛠️ 9. CI and Task Execution (`devx action`)

Do NOT manually run native build or test commands (like `npm run build`, `go test`, `cargo test`, `pytest`, etc.) yourself.

Any time you need to execute unit tests, integration tests, or verify compilation, use `devx`'s custom action runner by invoking predefined sequences defined in the project's `devx.yaml`:

- `devx action ci`: Executes the standard CI pipeline for this project.
- `devx action test`: Executes the standalone test suite (if defined).
- `devx action <action-name> --detailed`: Use this if a step fails and you need the full verbose terminal output (instead of the concise TUI summary) to debug the failure log.

These devx wrappers natively emit rich telemetry for Grafana observability and enforce deterministic test environments. Prefer project actions in `devx.yaml` so runs stay consistent and observable.

## 🚀 10. PR-First Review Workflow (`devx agent review` / `devx agent ship`)

Never commit or push directly to `main`. Use a Pull Request workflow from a dedicated feature branch.

If this repository has installed devx's git hooks (`devx agent ship --install-hook`), `.git/hooks/pre-commit` blocks direct commits to `main`/`master` and `.git/hooks/pre-push` blocks raw `git push`, routing changes through `devx agent review` or `devx agent ship`.

**Mandatory Branching Step:**
Before making ANY code edits or running any shipping commands, you MUST create and check out a dedicated feature branch from `main`:
```bash
git checkout -b feat/your-feature-name
```

**Standard Workflow (For Human Review):**
Unless the user explicitly asks you to auto-merge, you MUST always use `devx agent review`:
```bash
devx agent review -m "feat: your commit message"
```
This command orchestrates the full review lifecycle:
1. **Pre-flight**: Runs local tests, lint, and build automatically.
2. **Commit & Push**: Stages, commits, and pushes to the remote feature branch.
3. **PR Creation**: Automatically creates a GitHub Pull Request from your feature branch to `main`.
4. **CI Verification**: Polls the CI pipeline and blocks until it completes.
5. **Human Review**: Leaves the PR open for human review (does NOT auto-merge).

**For Automated Merging (Only when explicitly requested):**
If and only if the user explicitly instructs you to auto-merge the changes once green, use:
```bash
devx agent ship -m "feat: your commit message"
```
This performs the same steps but squash-merges the PR automatically once CI passes.

*Note: Running either command while checked out on `main` will fail, as GitHub cannot create a PR with `main` as both the head and base branch.*

## 🔗 11. Hybrid Bridge (`devx bridge`)

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

## 📦 12. State Replication (`devx state share` / `attach`)

Share and restore exact running container states and database volumes across machines.
State replication enforces a "Bring-Your-Own-Bucket" model using S3 or GCS.

### Commands

**Share State:**
- `devx state share --json`: Bundle current CRIU checkpoints and database volumes, encrypt, and upload to relay.
- `devx state share --db-only --json`: Skip container checkpoints (required for non-Podman users).
- `devx state share --relay s3://my-bucket/state`: Override `devx.yaml` relay destination.

**Attach State:**
- `devx state attach <ID> -y`: Download, decrypt, and restore the state bundle (overwrites current local state).

### Exit Codes
| Code | Meaning |
|------|---------|
| 80 | No running devx containers/databases to share |
| 81 | Failed to upload bundle to relay/bucket |
| 82 | Malformed share ID |
| 83 | Failed to download bundle |
| 84 | Wrong passphrase or corrupted bundle (decryption failed) |
| 85 | Checkpoint or snapshot restore failed |

## 🧬 13. AI Synthetic Data Generation (`devx db synthesize`)

Generate realistic, edge-case-heavy synthetic data by extracting the local database schema and sending it to an AI model. The generated SQL is sanitized and piped directly into the running container.

### Commands

- `devx db synthesize postgres`: Generate 100 synthetic records for PostgreSQL via the best available AI provider.
- `devx db synthesize mysql --records 50`: Generate 50 records for MySQL.
- `devx db synthesize postgres --model llama3`: Override the default LLM model.
- `devx db synthesize postgres --dry-run`: Preview the extracted schema and LLM prompt without generating data.
- `devx db synthesize postgres --json`: Return structured JSON output for programmatic consumption.

### AI Provider Priority
1. Local Ollama on port 11434 (zero-config)
2. Local LM Studio on port 1234
3. `OPENAI_API_KEY` environment variable (cloud fallback, also supports `OPENAI_API_BASE` for custom endpoints)

### Supported Engines
Only `postgres` and `mysql` support DDL extraction. Running against `redis` or `mongo` will fail with exit code 88.

### Exit Codes
| Code | Meaning |
|------|---------|
| 86 | No local LLM or cloud API key found |
| 87 | LLM API request failed or timed out |
| 88 | Engine does not support DDL extraction (mongo/redis) |
| 89 | Generated SQL failed to execute against the database |

## 🗣️ 14. Natural Language Database Queries (`devx db ask`)

Use natural language to query local databases. `devx` extracts the schema and translates the question into safe SQL. All queries are wrapped in a read-only transaction by default.

### Commands

- `devx db ask postgres "users who signed up this week"`
- `devx db ask mysql "show me the 5 largest tables"`

**Built-in Diagnostics (No AI needed):**
- `devx db ask postgres --sizes` (Table sizes and row counts)
- `devx db ask postgres --recent` (Last 10 rows from each table)
- `devx db ask postgres --missing-indexes` (Tables without indexes)
- `devx db ask postgres --nulls <table>` (Column NULL ratios)

### Flags

- `--dry-run`: Preview the generated SQL without executing it.
- `--json`: Get structured JSON output instead of the rendered TUI table.
- `--allow-writes`: Allow the generated query to perform write operations (must use `-y` or you will be blocked by a confirmation prompt).
- `--model`: Override the default LLM model.
- `--runtime`: Container runtime to use (`podman` or `docker`).

If the command fails due to no AI provider, the exit code will be `91`. Generated query failures result in `92`. Attempting a write without `--allow-writes` results in `93`.

## 📖 15. CLI Command & Flag Reference

<!-- BEGIN GENERATED COMMAND REFERENCE -->

_Auto-generated from the `devx` Cobra command tree (`go generate ./cmd`). Do not edit manually._

### Global Flags

| Flag | Description |
|------|-------------|
| `--detailed` | Enable detailed Go test output (shows individual passing tests instead of just package summaries) |
| `--dry-run` | Print what destructive actions would do without executing them |
| `--env-file <string>` | Path to secrets file (default: .env in current directory) |
| `--experimental` | Enable experimental (alpha) commands that depend on brittle third-party tools |
| `--json` | Output results in machine-readable JSON format for AI agents |
| `-y, --non-interactive` | Bypass interactive prompts and auto-confirm destructive actions |

### Commands

| Command | Flags | Summary |
|---------|-------|---------|
| `devx action [name]` | `--list` | Run a named custom action defined in devx.yaml |
| `devx agent` | — | Manage AI Agent configuration for your project |
| `devx agent init` | `-f, --force` | Install AI agent skills for the devx environment |
| `devx agent review` | `--ai-review`, `--base <string>`, `--ci-timeout <duration>`, `-m, --message <string>`, `--skip-preflight`, `-v, --verbose` | Commit, push, and create a PR for human review |
| `devx agent ship` | `--base <string>`, `--ci-timeout <duration>`, `--install-hook`, `-m, --message <string>`, `--skip-preflight`, `-v, --verbose` | Commit, push, and verify CI in a single blocking operation |
| `devx audit` | `--secrets`, `--vulns` | Scan for leaked secrets and dependency vulnerabilities before pushing |
| `devx audit install-hooks` | — | Install a git pre-push hook that runs devx audit automatically |
| `devx audit secrets` | — | Scan for hardcoded secrets and credentials (Gitleaks) |
| `devx audit vulns` | — | Scan dependencies for known CVEs (Trivy) |
| `devx bridge` | — | Connect your local environment to a remote Kubernetes cluster |
| `devx bridge connect` | `--context <string>`, `--kubeconfig <string>`, `-n, --namespace <string>`, `-t, --target <stringArray>` | Establish outbound bridge to remote cluster services |
| `devx bridge disconnect` | — | Tear down all active bridge sessions |
| `devx bridge intercept <service>` | `--agent-image <string>`, `--context <string>`, `--kubeconfig <string>`, `--local-port <int>`, `--mirror`, `-n, --namespace <string>`, `-p, --port <int>`, `--steal` | Intercept inbound traffic from a remote Kubernetes service |
| `devx bridge rbac` | `-n, --namespace <string>` | Print the minimum RBAC manifest required for bridge intercept |
| `devx bridge status` | — | Show active bridge sessions |
| `devx ci` | — | Local CI pipeline emulation |
| `devx ci run [workflow.yml]` | `--image <string>`, `--job <stringSlice>`, `--runtime <string>` | Execute a GitHub Actions workflow locally |
| `devx cloud` | — | Spawn local GCP cloud service emulators [experimental] |
| `devx cloud list` (aliases: ls) | `--runtime <string>` | List all devx-managed cloud emulators |
| `devx cloud rm <service>` | `--runtime <string>` | Stop and remove a devx-managed cloud emulator |
| `devx cloud spawn <service>` | `-p, --port <int>`, `--runtime <string>` | Spawn a local GCP service emulator |
| `devx cluster` | `-c, --config <string>` | Provision and manage multi-node K8s clusters across macOS machines |
| `devx cluster apply` | — | Apply configuration changes iteratively with zero downtime |
| `devx cluster backup` | `-o, --output <string>`, `--timeout <duration>` | Create an etcd snapshot backup |
| `devx cluster destroy` | `--timeout <duration>` | Tear down the entire cluster |
| `devx cluster doctor` | `--fix` | Run pre-flight checks and health diagnostics |
| `devx cluster init` | `--auto-install`, `--timeout <duration>` | Bootstrap a new cluster from the config file |
| `devx cluster join` | `--timeout <duration>` | Add new node(s) to an existing cluster |
| `devx cluster reconcile` | `--timeout <duration>` | Converge existing nodes to the current node baseline (e.g. required packages) |
| `devx cluster remove <node-host>` | `--timeout <duration>` | Drain and remove a node from the cluster |
| `devx cluster restore` | `-s, --snapshot <string>`, `--timeout <duration>` | Restore from an etcd snapshot |
| `devx cluster status` | — | Show the current cluster state |
| `devx cluster upgrade` | `--timeout <duration>` | Rolling upgrade K3s to the version specified in config |
| `devx cluster usb` | — | Build a USB that boots a laptop into a self-joining node |
| `devx cluster usb build` | `--assemble`, `--builder-vm <string>`, `--device <string>`, `--image-out <string>`, `--image-size <string>`, `--mode <stringSlice>`, `-o, --output <string>`, `--renderer <stringSlice>`, `--role <string>`, `--timeout <duration>`, `--yes` | Generate boot artifacts, or assemble + flash a bootable FCOS USB |
| `devx cluster usb prune` | `--timeout <duration>`, `--ttl <duration>` | Remove NotReady ephemeral USB nodes from the cluster (ghost reaper) |
| `devx config` | — | Manage devx configuration and credentials |
| `devx config pull` | — | Pull team-managed `.env` secrets from vault providers securely. |
| `devx config push` | — | Push local `.env` secrets up to team-managed remote vaults. |
| `devx config secrets` | — | Add or rotate credentials stored in .env |
| `devx config validate` | `--schema <string>` | Validate environment variables against a schema before starting the app |
| `devx cron` | — | Run and inspect project-defined cron jobs locally |
| `devx cron list` | — | List cron jobs declared in devx.yaml |
| `devx cron run <name>` | — | Execute a single cron job now, with the project's secrets and env |
| `devx db` | — | Manage local development databases |
| `devx db ask <engine> [question]` | `--allow-writes`, `--missing-indexes`, `--model <string>`, `--nulls <string>`, `--recent`, `--runtime <string>`, `--sizes` | Query your local database using natural language or built-in diagnostics |
| `devx db list` (aliases: ls) | `--runtime <string>` | List all devx-managed databases |
| `devx db pull <engine>` | `-j, --jobs <int>`, `--runtime <string>` | Pull scrubbed production data and stream it into the local database |
| `devx db restart <engine>` | `--runtime <string>` | Restart a devx-managed database container (volume data is preserved) |
| `devx db rm <engine>` | `--keep-volume`, `--project <string>`, `--runtime <string>` | Stop and remove a devx-managed database |
| `devx db seed <engine>` | `--runtime <string>` | Run the configured seed command against the local database |
| `devx db snapshot` | — | Manage point-in-time snapshots of local database volumes |
| `devx db snapshot create <engine> <name>` | — | Create a named snapshot of a database volume |
| `devx db snapshot list <engine>` | — | List available snapshots for a database engine |
| `devx db snapshot restore <engine> <name>` | — | Restore a database volume from a named snapshot |
| `devx db snapshot rm <engine> <name>` | — | Delete a named snapshot |
| `devx db spawn <engine>` | `-p, --port <int>`, `--project <string>`, `--runtime <string>` | Spin up a local database with persistent storage |
| `devx db synthesize <engine>` | `--model <string>`, `--records <int>`, `--runtime <string>` | Generate realistic synthetic data using AI |
| `devx doctor` | — | Check that all prerequisites are installed and configured |
| `devx doctor auth` | — | Walk through authenticating required tools and credentials |
| `devx doctor install` | `--all` | Install missing prerequisite CLI tools |
| `devx exec` | — | Run low-level infrastructure tools directly |
| `devx exec butane [args...]` | _(passthrough)_ | Pass-through command for local Butane utility. |
| `devx exec cloudflared [args...]` | _(passthrough)_ | Pass-through command for local Cloudflared utility. |
| `devx exec podman [args...]` | _(passthrough)_ | Pass-through command for local Podman executing against the VM. |
| `devx exec tailscale [args...]` | _(passthrough)_ | Pass-through command for Tailscale interacting explicitly with the VM daemon. |
| `devx k8s` | — | Zero-config local Kubernetes clusters powered by k3s [experimental] |
| `devx k8s list` (aliases: ls) | `--runtime <string>` | List all running k3s clusters |
| `devx k8s rm <name>` | `--runtime <string>` | Stop and remove a k3s cluster |
| `devx k8s spawn [name]` | `--runtime <string>` | Spin up a zero-config local Kubernetes cluster |
| `devx logs` | — | Unified TUI log multiplexer for containers and native host processes via BubbleTea |
| `devx mail` | — | Manage local email capture and inspection services [experimental] |
| `devx mail list` (aliases: ls) | `--runtime <string>` | List the devx-managed mail catcher |
| `devx mail rm` | `--runtime <string>` | Stop and remove the devx-managed mail catcher |
| `devx mail spawn` | `--runtime <string>`, `-p, --smtp-port <int>`, `--ui-port <int>` | Spawn a local SMTP email catcher (MailHog) |
| `devx map` | `-o, --output <string>` | Generate a visual architecture map from devx.yaml as a Mermaid diagram |
| `devx mcp` | — | Expose devx as a Model Context Protocol (MCP) server for AI coding agents |
| `devx mcp doctor` | — | Validate the devx MCP server starts and lists tools correctly |
| `devx mcp install [host...]` | `--all`, `--global` | Install the devx MCP server into one or more agent hosts |
| `devx mcp list` | — | List supported agent hosts and MCP tools |
| `devx mcp serve` | `--list-tools` | Run the devx MCP server over stdio (invoked by agent hosts) |
| `devx mcp status` | — | Show where the devx MCP server is currently installed |
| `devx mcp uninstall [host...]` | `--global` | Remove the devx MCP server from one or more agent hosts |
| `devx mock` | — | Manage local OpenAPI mock servers powered by Stoplight Prism [experimental] |
| `devx mock list` (aliases: ls) | `--runtime <string>` | List all running devx mock servers |
| `devx mock restart <name>` | `--runtime <string>` | Restart a running mock server (re-fetches the OpenAPI spec on boot) |
| `devx mock rm <name>` | `--runtime <string>` | Stop and remove a mock server |
| `devx mock up [name...]` | `--port <int>`, `--runtime <string>`, `--url <string>` | Start mock servers defined in devx.yaml (or specify names inline) |
| `devx nuke` | `--runtime <string>` | Hard reset: purge caches, builds, and devx resources for this project |
| `devx preview <pr-number>` | — | Spin up an isolated sandbox to review a PR without switching branches |
| `devx run -- [command...]` | `-n, --name <string>` | Run a command with telemetry, secret injection, and log routing |
| `devx scaffold [template] [project-name]` | `--author <string>`, `--desc <string>`, `--dir <string>`, `--domain <string>`, `-f, --force`, `--module <string>`, `--no-git` | Generate a new project from a pre-wired template |
| `devx scaffold list` | — | List all available scaffold templates |
| `devx shell` | — | Launch an isolated dev shell from devcontainer.json |
| `devx sites` | — | Manage documentation and static site hosting |
| `devx sites init` | `--domain <string>`, `--name <string>` | Enable GitHub Pages and configure Cloudflare DNS for this repository |
| `devx sites status` | — | Check GitHub Pages deployment and SSL certificate status |
| `devx sites verify` | `--domain <string>` | Create a DNS TXT record for GitHub Pages domain verification |
| `devx state` | — | Manage topological state (diagnostic dumps, checkpoints, and restores) |
| `devx state attach <ID>` | — | Download and restore a shared environment state from a teammate |
| `devx state checkpoint <name>` | — | Snapshot the entire running topology's RAM and volumes using CRIU |
| `devx state dump` | `-f, --file <string>` | Snapshot topology, dependencies, and crashing container logs into a Shareable Diagnostic Dump |
| `devx state list` | — | List all time-travel checkpoints |
| `devx state restore <name>` | — | Restore all running topology containers from a time-travel checkpoint |
| `devx state rm <name>` | — | Delete a time-travel checkpoint |
| `devx state share` | `--db-only`, `--relay <string>` | Bundle and share your running environment state with a teammate |
| `devx stats` | `--clear` | Display local devx performance metrics (P50/P90/P99 latency) |
| `devx sync` | — | Manage intelligent file synchronization into containers |
| `devx sync list` | — | List active file sync sessions |
| `devx sync rm [names...]` | — | Terminate file sync sessions |
| `devx sync up [names...]` | `--runtime <string>` | Start file sync sessions defined in devx.yaml |
| `devx test` | — | Run isolated test topologies against ephemeral environments |
| `devx test ui [-- command]` | `--command <string>`, `--runtime <string>`, `--setup <string>` | Run UI/E2E tests against an ephemeral, isolated topology |
| `devx trace` | — | Manage local distributed tracing and observability backends |
| `devx trace dashboard` | — | Provision the devx Build Metrics dashboard to the configured Grafana instance |
| `devx trace list` (aliases: ls) | `--runtime <string>` | List all devx-managed telemetry backends |
| `devx trace rm <engine>` | `--runtime <string>` | Stop and remove a devx-managed telemetry backend |
| `devx trace spawn [engine]` | `--persist`, `--runtime <string>` | Spawn a local distributed tracing backend |
| `devx tunnel` | — | Manage Cloudflare tunnels and port exposure |
| `devx tunnel expose [port]` | `--basic-auth <string>`, `--domain <string>`, `-n, --name <string>`, `--throttle <string>` | Expose a local port to the internet via your *.ipv1337.dev domain. |
| `devx tunnel inspect [port]` | `--basic-auth <string>`, `--domain <string>`, `-e, --expose`, `-n, --name <string>` | Inspect and replay HTTP traffic with a live TUI |
| `devx tunnel list` (aliases: exposures, ls) | — | List active port exposures |
| `devx tunnel unexpose` | — | Clean up all exposed localhost ports routed through Cloudflare. |
| `devx tunnel up` | `--domain <string>`, `--logs`, `--no-logs`, `--profile <string>` | Provision databases and expose ports defined in devx.yaml. |
| `devx tunnel update` | — | Rotate Cloudflare credentials without rebuilding the VM |
| `devx up` | `--domain <string>`, `--logs`, `--no-logs`, `--profile <string>` | Provision databases and expose ports defined in devx.yaml. |
| `devx upgrade` | — | Upgrade devx to the latest release |
| `devx version` | — | Print the devx version and check for updates |
| `devx vm` | `--provider <string>` | Manage the local development VM |
| `devx vm init` | — | Provision the local dev environment |
| `devx vm resize` | `-c, --cpus <int>`, `-m, --memory <int>` | Dynamically resize VM RAM and CPU allocation |
| `devx vm sleep` | — | Put the VM to sleep (pause resources) |
| `devx vm sleep-watch` | `-i, --interval <int>` | Run a background daemon to auto-sleep idle VMs |
| `devx vm ssh [args...]` | _(passthrough)_ | Drop into an SSH shell inside the VM. |
| `devx vm status` | — | Show the health of the VM, Cloudflare tunnel, and Tailscale |
| `devx vm teardown` | `-f, --force` | Stop and remove the dev VM (destructive) |
| `devx webhook` | — | Local webhook tools |
| `devx webhook catch` | `--expose`, `-p, --port <int>`, `--runtime <string>` | Capture and inspect outbound webhook payloads in the terminal |

<!-- END GENERATED COMMAND REFERENCE -->
