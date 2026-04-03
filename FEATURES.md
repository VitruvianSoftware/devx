# devx Features

This document serves as an organized historical record of all major capabilities shipped in `devx`. These features were designed to break down traditional paywalls (like those found in ngrok) and make local development hyper-extensible for any team. All items below correspond to their original "Idea Number" from the project roadmap.
## The "Ngrok Alternatives" (Paywall Breakers)

### 1. Local Request Inspector & Webhook Replay (TUI) (DONE)
* **The Problem:** Ngrok's web inspector allows devs to see incoming HTTP requests and manually replay them. This is crucial for webhook development but is heavily commercialized.
* **The Solution:** Build a beautiful Bubble Tea terminal UI (`devx tunnel inspect`). It acts as a local proxy that intercepts, displays, and allows one-click replaying of HTTP requests coming through the Cloudflare tunnel—right in your terminal.

### 2. Built-in Authentication & Access Control (DONE)
* **The Problem:** Putting Basic Auth, OAuth, or IP whitelisting on an exposed tunnel usually requires an expensive subscription.
* **The Solution:** Add simple flags like `devx tunnel expose 8080 --basic-auth "user:pass"` or `--github-org "VitruvianSoftware"`. Because we leverage Cloudflare, we can automatically configure Cloudflare Access API to enforce zero-trust policies dynamically on edge URLs for free.

### 3. Multi-Port Mapping via Project Configs (`devx.yaml`) (DONE)
* **The Problem:** Running multiple tunnels (e.g., frontend, backend, webhooks) requires opening multiple terminal tabs and managing them manually.
* **The Solution:** Read a `devx.yaml` file in the project root that defines a local topology. A developer runs `devx tunnel up`, and it simultaneously exposes all defined ports under unified, deterministic subdomains.

### 4. Custom Domain Support (BYOD) (DONE)
* **The Problem:** Using a custom, branded domain name (e.g., `api.mycompany.dev`) instead of a random subdomain is a premium-only feature in most tools.
* **The Solution:** Allow users to bring their own Cloudflare zones to the CLI. Pass `--domain mycompany.dev` to provision custom edges effortlessly and permanently.

---

## Developer Productivity & Extensibility

### 5. Backend Pluggability (OrbStack / Docker Support) (DONE)
* **The Problem:** Hardcoding Fedora CoreOS + Podman Machine is highly opinionated. Some devs might already have Docker Desktop or OrbStack running and don't want to run two VMs.
* **The Solution:** Abstract the `internal/podman` package into a `VirtualizationProvider` interface. Allow developers to map `devx` networking (Cloudflare + Tailscale) directly on top of their existing hypervisor (`--provider=orbstack`).

### 6. Built-in Dev Containers (`devcontainer.json`) Integration (DONE)
* **The Problem:** The VM gives you an OS, but you still need node, go, rust, etc. for your specific project.
* **The Solution:** Add native support for `devcontainer.json`. Running `devx shell` reads the config, spins up the exact container inside the Podman VM, mounts your code, and drops you into a shell with completely isolated tooling.

### 7. One-Click Database Provisioning (`devx db spawn`) (DONE)
* **The Problem:** Setting up local databases (Postgres, Redis), persisting data, and wiring them securely takes friction.
* **The Solution:** Add `devx db spawn postgres`. This spins up Postgres inside the VM, wires it to the Tailnet automatically, and prints the connection string. It guarantees persistence across VM rebuilds using core Podman volumes.

### 8. Network Simulation (Traffic Shaping & Fault Injection) (DONE)
* **The Problem:** Testing how your local app handles high latency, dropped packets, or 3G network speeds is extremely tedious.
* **The Solution:** Add `--throttle=3g` or `--latency=200ms` flags to the `expose` command. We leveraged a native Go TCP proxy (`internal/trafficproxy`) to intercept the Cloudflare tunnel and artificially shape incoming Edge traffic. This allows cross-platform simulation (macOS/Docker/OrbStack) for frontend/mobile testing without OS-level `tc` limits.

### 9. Automated Resource Scaling & Deep Sleep (DONE)
* **The Problem:** VMs reserve RAM and CPU constantly, draining Macbook batteries when sitting idle.
* **The Solution:** Implemented auto-pause/deep-sleep (`devx vm sleep-watch`) when no containers are active. We also added seamless JIT wake-ups. VM will automatically resume the moment you invoke any `devx` commands that require infrastructure. Finally, we added support to dynamically resize the VM (`devx vm resize --cpus 4 --memory 8192`) without destroying the machine context.

### 10. Global Secret Sync & `.env` Management (DONE)
* **The Problem:** Sharing `.env` files across a team securely is a massive pain, often resulting in Slack DMs and out-of-sync configurations.
* **The Solution:** Integrated with native vault providers (1Password CLI, Bitwarden, and GCP Secret Manager). Define your vaults in `devx.yaml: env:` and they will instantly and transparently inject into the Podman/Docker app memory during `devx shell`, bypassing plaintext files on the Macbook disk. If local `.env` values are preferred, developers can mix-and-match multiple vaults or just fall back to `file://.env`. Running `devx config pull` fetches all references into local variables.

---

## Support Agentic AI Development

### 11. Agentic Output Context (`--json`) (DONE)
* **The Problem:** AI dev agents (like GitHub Copilot Workspaces, Devin, or local CLIs) drop state tracking when parsing messy human-formatted ascii tables or colored output.
* **The Solution:** Implemented a global `--json` flag to export machine-readable deterministic JSON output across all major informational commands like `devx vm status`, `devx tunnel list`, and `devx db list`.

### 12. Non-blocking UI Bypass (`--non-interactive`) (DONE)
* **The Problem:** AI agents easily get stuck forever waiting on invisible TUI/CLI prompts (like survey forms or confirmation warnings) that they have no mechanism to "press enter" on.
* **The Solution:** Added a strict `--non-interactive` (or `-y`) flag globally. This forces `devx` to instantly accept safe defaults, bypass teardown/deletion confirmation warnings automatically, or hard-fail immediately if requirements aren't met rather than stalling inside the hidden `huh` TUI forms.

### 13. Destructive Action Preflight (`--dry-run`) (DONE)
* **The Problem:** Agents are prone to hallucinating or misunderstanding their scope. If an agent calls `devx vm teardown` or `devx db rm`, it could wipe out gigabytes of critical developer state irreversibly.
* **The Solution:** Added a global `--dry-run` flag wired into destructive actions (`vm teardown`, `db rm`, `tunnel unexpose`) so the agent can safely perform a preflight dry-run check and deterministically echo what records, VMs, and data would be deleted BEFORE actually taking action or bypassing the interactive prompts.

### 14. Standardized Predictable Exit Codes (DONE)
* **The Problem:** Command Line tools typically just return `exit status 1` for every single failure under the sun, forcing AI agents to parse raw English stderr logs to figure out what broke.
* **The Solution:** Mapped specific internal devx errors into specific deterministic exit status codes (e.g. `Exit Code 22: Port Address in Use`, `Exit Code 15: VM Dormant`, `Exit Code 41: Cloudflare Missing Auth Credentials`). We also disabled `--help` spam on failure. This ensures agents parsing `devxerr` exits can cleanly `switch(exitCode)` to write tight programmatic rescue paths without LLM parsing hallucination risks.

### 15. Official Agent Tool Manifest (`.agent/skills/devx`) (DONE)
* **The Problem:** When an AI drops into a repository, it has to guess what `devx` does or blindly run `devx --help` over and over, consuming expensive tokens.
* **The Solution:** Created official agent manifest files (`.agent/skills/devx/SKILL.md` for standard Antigravity/Gemini agents, `.cursorrules` for Cursor, `CLAUDE.md` for Claude Code, and `.github/copilot-instructions.md` for Copilot). These configuration files ensure any AI immediately drops into a repository natively understanding `devx`'s non-interactive composition, JSON workflows, and numeric error codes without hallucinations.

---

## Developer Onboarding Automation

### 16. `devx doctor` — Prerequisite Auditor & Auto-Installer

The goal is to eliminate **all** onboarding friction by providing a single `devx doctor` command that audits, installs, and configures every tool and credential that `devx` depends on. Developers should be able to clone the repo, run `devx doctor`, and be fully operational — zero manual steps.

#### Task 16a: Prerequisite Audit (`devx doctor check`) (DONE)
* **Status:** DONE
* **What:** Add a `devx doctor` command (aliased as `devx doctor check`) that audits every external CLI tool and reports what's installed, what's missing, and what version is detected.
* **Details:**
  - Detect which tools are present via `exec.LookPath()` or equivalent
  - Report version where possible (`podman --version`, `gh --version`, etc.)
  - Color-coded output: ✅ installed, ❌ missing, ⚠️ outdated
  - Group tools by feature area (Core, Tunnels, Sites, Vault, etc.)
* **Tools to audit:**

  | Tool | Feature Area | Required? | Install Command |
  |------|-------------|-----------|-----------------|
  | `podman` | Core (VM) | Yes (if `--provider=podman`) | `brew install podman` |
  | `docker` | Core (VM) | Yes (if `--provider=docker`) | `brew install docker` |
  | `orb` | Core (VM) | Yes (if `--provider=orbstack`) | `brew install orbstack` |
  | `cloudflared` | Tunnels | Yes | `brew install cloudflare/cloudflare/cloudflared` |
  | `butane` | VM Init | Yes | `brew install butane` |
  | `gh` | Sites | Yes (for `sites` commands) | `brew install gh` |
  | `op` | Vault (1Password) | Optional | `brew install 1password-cli` |
  | `bw` | Vault (Bitwarden) | Optional | `brew install bitwarden-cli` |
  | `gcloud` | Vault (GCP) | Optional | `brew install google-cloud-sdk` |

#### Task 16b: System & Package Manager Detection (DONE)
* **Status:** DONE — implemented as part of `DetectSystem()` in `internal/doctor/check.go`. Detects macOS/Linux, architecture, and package manager (brew, apt, dnf, pacman, yum, apk, nix).
* **What:** Detect the developer's OS (macOS, Linux distro) and available package managers.
* **Details:**
  - macOS: prefer `brew`, detect if Homebrew is installed, offer to install it if missing
  - Linux: detect `apt`, `dnf`, `pacman`, `yum`, `apk`, `nix`
  - Map each prerequisite tool to install commands for each detected package manager
  - Store detection results for use by the auto-installer (Task 16c)

#### Task 16c: Auto-Installer (`devx doctor install`) (DONE)
* **Status:** DONE
* **What:** Offer to install missing tools automatically using the detected package manager.
* **Details:**
  - Interactive mode: present a checklist of missing tools, let the developer select which to install
  - Non-interactive mode (`-y`): install all missing required tools
  - Show the exact commands being run before executing (transparency)
  - Handle tap/repository additions (e.g., `brew tap cloudflare/cloudflare` before installing `cloudflared`)
  - Summarize what was installed and any post-install steps needed

#### Task 16d: Credential & Authentication Audit (DONE)
* **Status:** DONE — implemented as part of `CheckCredentials()` in `internal/doctor/check.go`. Checks Cloudflare API token, cloudflared cert, GitHub CLI auth (with scope detection), Tailscale VM state, CF tunnel token, and optional vault credentials (1Password, Bitwarden, GCP with multi-account support).
* **What:** Check that required credentials and authentication sessions are configured.
* **Details:**
  - Check for each credential and report status:

  | Credential | Source | Required For | How To Check |
  |-----------|--------|-------------|-------------|
  | `CLOUDFLARE_API_TOKEN` | `.env` or env var | `sites`, DNS operations | Non-empty + basic API test |
  | `CF_TUNNEL_TOKEN` | Generated by `cloudflared` | `tunnel expose` | Exist check |
  | `gh` auth session | `gh auth status` | `sites init/status` | `gh auth status` exit code |
  | `gh` admin:org scope | `gh auth status` | `sites init` (org repos) | Check scopes in output |
  | `cloudflared` login | `~/.cloudflared/cert.pem` | `vm init`, tunnel creation | File existence |
  | Tailscale auth key | `.env` or Ignition | `vm init` (Tailnet join) | Exist check |
  | `op` session | `op account list` | Vault (1Password) | Exit code |
  | `bw` session | `bw status` | Vault (Bitwarden) | JSON output parse |
  | `gcloud` auth | `gcloud auth print-access-token` | Vault (GCP) | Exit code |

#### Task 16e: Guided Authentication (`devx doctor auth`) (DONE)
* **Status:** DONE — implemented in `internal/doctor/auth.go`. Walks through cloudflared login, gh auth (with scope refresh), and Cloudflare API token. Skips already-configured steps.

#### Task 16f: Unified `devx doctor` Dashboard (DONE)
* **Status:** DONE — the base `devx doctor` command already runs the full audit: system info, CLI tools, credentials, and feature readiness.
* **What:** Combine all checks into a single beautiful dashboard output.
* **Details:**
  - When run without a subcommand, `devx doctor` runs the full audit:
    1. System info (OS, arch, package manager)
    2. CLI tools (installed/missing/version)
    3. Credentials (configured/missing/expired)
    4. Feature readiness (which `devx` features are fully operational)
  - Example output:
    ```
    🩺 devx doctor — Environment Health Check

    System:   macOS 15.3 (arm64) • Homebrew 4.x

    CLI Tools:
      ✅ podman     4.9.4    (Core VM)
      ✅ cloudflared 2024.12  (Tunnels)
      ✅ butane      0.22.0   (Ignition)
      ✅ gh          2.65.0   (Sites)
      ❌ op          —        (1Password Vault) [optional]

    Credentials:
      ✅ Cloudflare API Token  configured (.env)
      ✅ cloudflared login     ~/.cloudflared/cert.pem
      ✅ GitHub CLI            authenticated (admin:org ✓)
      ⚠️  Tailscale auth key   not found (.env)

    Feature Readiness:
      ✅ devx vm init          ready
      ✅ devx tunnel expose    ready
      ✅ devx sites init       ready
      ✅ devx db spawn         ready
      ⚠️  devx config pull      needs: op or bw or gcloud
    ```

#### Task 16g: Update README & Documentation (DONE)
* **Status:** DONE
* **What:** Update the README prerequisites section and the docs site to reference `devx doctor` instead of manual install instructions.
* **Details:**
  - README Quick Start: `devx doctor` as step 0 before `devx vm init`
  - Getting Started docs page: replace manual prerequisite table with `devx doctor` workflow
  - Add a new docs page: `guide/doctor.md` with full reference for all doctor subcommands
  - Update VitePress sidebar config to include the new page

---

## Advanced Local Development Experience (Next Gen)

### 17. Unified Container Log Multiplexer (TUI) (DONE)
* **The Problem:** When running a microservices architecture locally, developers have to open 5-10 terminal tabs running `docker logs -f` just to trace a single request through the API, worker, and database.
* **The Solution:** Implemented `devx logs` — a Bubble Tea TUI that automatically discovers all running Podman containers and any native host processes started via `devx run`, multiplexes their stdout/stderr into a single unified stream, and color-codes them by service name. Also supports `--json` for AI agent consumption.
* **Key files:** `cmd/logs.go`, `cmd/run.go`, `internal/logs/streamer.go`, `internal/logs/tui.go`

### 18. Instant Database Snapshot & Restore (DONE)
* **The Problem:** Developers often need to test destructive database migrations or complex state changes. If they mess up, resetting the local DB to a clean state takes too long (dropping tables, running seeds).
* **The Solution:** Implemented `devx db snapshot create <engine> <name>` and `devx db snapshot restore <engine> <name>`. Uses `podman volume export/import` (tar archives) for instant, zero-SQL snapshots. For Docker, falls back to a lightweight Alpine helper container. Full subcommand tree: `create`, `restore`, `list`, `rm`. Respects `--dry-run`, `-y`, and `--json` flags.
* **Key files:** `cmd/db_snapshot.go`, `internal/database/snapshot.go`

### 19. Local GCS & Cloud Emulation (DONE)
* **The Problem:** Modern apps depend on GCP cloud services (GCS for file uploads, Pub/Sub for queues). Testing these locally requires pointing to real GCP projects or shared dev environments that get polluted.
* **The Solution:** Implemented `devx cloud spawn gcs|pubsub|firestore`. Runs `fake-gcs-server` (GCS), or the GCP Cloud SDK emulators (Pub/Sub, Firestore) as named containers. Automatically injects the correct SDK env vars (`STORAGE_EMULATOR_HOST`, `PUBSUB_EMULATOR_HOST`, etc.) when using `devx shell`. Full subcommand tree: `spawn`, `list`, `rm`. Respects `--json` and `--dry-run`.
* **TODO:** AWS S3 emulation via MinIO (`devx cloud spawn s3`) planned for a future release.
* **Key files:** `cmd/cloud.go`, `cmd/cloud_spawn.go`, `cmd/cloud_list.go`, `cmd/cloud_rm.go`, `internal/cloud/emulator.go`

### 20. Zero-Config Local HTTPS / TLS Certificates _(SUPERSEDED)_
* **The Problem:** Dealing with mixed-content unsecure warnings (`http://localhost`) when integrating with APIs (like Stripe webhooks or OAuth flows) that strictly require `https://`.
* **Why it's superseded:** `devx tunnel expose` already solves the primary use case. Every exposed port gets a CA-signed `*.ipv1337.dev` public URL with HTTPS managed entirely by Cloudflare — no `mkcert`, no local CA, no certificate rotation. Stripe, OAuth providers, GitHub webhooks, and any other service requiring HTTPS can all use the tunnel URL directly.
* **Remaining edge case:** Pure local service-to-service mTLS (no public hop) is not covered by tunnels. If this becomes a real developer pain point, `devx certs provision` (local CA + wildcard `*.devx.local` certs) can be revisited as a scoped addition. For now the Cloudflare tunnel is the recommended path.

### 21. Automated Environment Variable Validation (DONE)
* **The Problem:** Developers pull `.env` files using `devx config pull`, start the app, and it crashes 3 minutes later because a new required variable was added by another developer but wasn't synced to their vault or local file.
* **The Solution:** Implemented `devx config validate`. Reads required keys from `.env.example` or `.env.schema`, fetches actual values from vault sources in `devx.yaml` (1Password, Bitwarden, GCP) or falls back to `.env`, and reports each key as ✓ ok / ✗ missing / ⚠ empty. Exits non-zero on failure so it integrates cleanly into CI. Respects `--json` for AI agents.
* **Key files:** `cmd/config_validate.go`

### 22. The "Nuke It" Button (Hard Project Reset) (DONE)
* **The Problem:** "Have you tried turning it off and on again?" When local caches become deeply corrupted (node_modules, Go caches, Podman dangling images, corrupted DB volumes), developers waste hours manually deleting directories.
* **The Solution:** Implemented `devx nuke`. Scans the project directory and devx runtime, builds a manifest of everything that would be deleted (grouped by language, with disk sizes), displays it alongside an explicit "Safe (never touched)" list, then uses a Huh confirmation form before performing atomic deletions. Supports `--dry-run`, `-y`, and `--runtime`. Covers Node.js, Go, Python, Rust, Java, and all devx-managed containers/volumes.
* **Key files:** `cmd/nuke.go`, `internal/nuke/nuke.go`

### 23. Local Email Catcher & Inspector (DONE)
* **The Problem:** Testing transactional emails locally risks accidentally emailing real users or requires developers to sign up for external services like Mailtrap.
* **The Solution:** Implemented `devx mail spawn`. Starts MailHog (`docker.io/mailhog/mailhog`) as a named devx-managed container with SMTP on port 1025 and a web UI on port 8025. `devx shell` automatically injects `SMTP_HOST`, `SMTP_PORT`, and `MAIL_CATCHER_URL` into the dev container. Also provides `devx mail list` and `devx mail rm`. MailHog exposes a JSON API at `/api/v2/messages` for automated test assertions.
* **Key files:** `cmd/mail.go`, `cmd/mail_spawn.go`, `cmd/mail_list.go`, `cmd/mail_rm.go`

### 24. Outbound Webhook Catcher & Request Bin (DONE)
* **The Problem:** We already solved *incoming* webhooks via Cloudflare Tunnels, but when the local app needs to *send* a webhook payload to a 3rd party, inspecting exactly what headers and JSON was sent often requires setting up a remote RequestBin in the browser.
* **The Solution:** Implemented `devx webhook catch`. A native Go HTTP server (no container needed) that accepts every request and displays it in a live Bubble Tea TUI with per-method colour coding (GET/POST/PUT/PATCH/DELETE), timestamp + duration, important signature header extraction (Stripe-Signature, X-Hub-Signature, X-Twilio-Signature, etc.), and pretty-printed syntax-highlighted JSON. Falls back to streaming JSON lines when not in a TTY (CI/jq). `--expose` wraps via Cloudflare tunnel for a public HTTPS URL.
* **Key files:** `cmd/webhook_catch.go`, `internal/webhook/server.go`, `internal/webhook/tui.go`

### 25. Secure Production Data Anonymization & Pull (DONE)
* **The Problem:** `devx db spawn` offers a blank database, which is useless for fixing a bug that only occurs with specific production data shapes. Dumping production data locally is a massive security/compliance risk.
* **The Solution:** Implemented `devx db pull <engine>`. Instead of re-implementing an insecure local scrubbing engine, `devx` delegates to the cloud: it parses a simple shell command defined in `devx.yaml` (e.g., pulling a nightly scrubbed dump from an S3 bucket). It ensures the local database is running and securely streams the pre-anonymized output straight into the database container's ingestion tool (`psql`, `mysql`, `mongorestore`, `redis-cli`) without writing massive temp files to disk.
* **Key files:** `cmd/db_pull.go`, `devx.yaml.example`

### 26. Instant Vulnerability & Secret Scanning (DONE)
* **The Problem:** Developers accidentally commit `.env` files or introduce NPM/Go vulnerabilities, which are only caught much later by GitHub Actions CI, breaking the build and requiring a context-switching PR fix.
* **The Solution:** Implemented `devx audit`. Runs Gitleaks (secret detection) and Trivy (CVE dependency scanning) against the working directory. Both tools auto-detect whether to run natively (if installed) or via an ephemeral read-only container mount — zero installation required. `devx audit install-hooks` wires it into `git pre-push` so pushes are blocked if issues are found. Supports `--secrets`, `--vulns`, `--json`, and `--runtime` flags.
* **Key files:** `cmd/audit.go`, `internal/audit/audit.go`

### 27. Service Scaffolding & Internal Developer Platforms (IDPs) (DONE)
* **The Problem:** `devx doctor` sets up the prerequisite tools, but getting a new microservice off the ground (frameworks, Dockerfiles, linting, CI config) requires manually copying from other repos, slowing down new development.
* **The Solution:** Implemented `devx scaffold <template>`. Using an embedded Template Registry, instantly generates a paved-path repository (e.g. `go-api`, `node-api`, `python-api`) pre-wired with standard CI pipelines, database migrations, dev seed data, and `devx.yaml` configurations. Fully supports interactive TUI forms (`huh`), non-interactive agent execution (`-y`), JSON output (`--json`), and idempotent re-executions.
* **Key files:** `cmd/scaffold.go`, `internal/scaffold/engine.go`

### 28. Zero-Friction Local AI Bridge (DONE)
* **The Problem:** Developers building AI applications struggle with the overhead of running models locally inside VMs while retaining GPU acceleration, and they often lose agent identities (`claude`, `opencode`) when entering isolated containers.
* **The Solution:** Implemented a lightweight AI Bridge that automatically detects host-level inference engines (Ollama, LM Studio) and natively injects `OPENAI_API_BASE` overrides into `devx shell`. Additionally bridges agentic workflow gaps by natively mounting identity/auth tokens, global skill vaults, Docker socket (DooD) sandboxing privileges, and SSH/Git forwarding capabilities seamlessly into the workspace.

### 29. Shift-Left Distributed Observability (DONE)
* **The Problem:** When running 5 microservices locally via `devx.yaml`, figuring out *where* a request failed requires tailing 5 sets of logs. Full distributed tracing is currently reserved for cloud/production because setting up an OTLP collector + Jaeger locally is too tedious.
* **The Solution:** Implemented `devx trace spawn [engine]`. Instantly spins up a lightweight OpenTelemetry backend (`jaeger` or `grafana` LGTM stack) locally. Auto-injects `OTEL_EXPORTER_OTLP_ENDPOINT` directly into all running `devx shell` and managed containers. Provides developers with immediate visual access to their local distributed traces.
* **Key files:** `cmd/trace.go`, `internal/telemetry/otel.go`, `cmd/shell.go`
* **Verification Proof:**
  * Jaeger Stack:
    ![Jaeger trace search — 1 Trace found](/Users/james/.gemini/antigravity/brain/ed2bc556-c685-48f3-a96d-f9e02ab64feb/jaeger_search_results_1775089747605.png)
  * Grafana LGTM Stack:
    ![Grafana Tempo Detail — duration 5.02s](/Users/james/.gemini/antigravity/brain/ed2bc556-c685-48f3-a96d-f9e02ab64feb/grafana_tempo_trace_details_1775093238026.png)

### 30. Ephemeral E2E Browser Testing Environments (DONE)
* **The Problem:** Running Cypress or Playwright tests locally destroys the developer's active database state or fights with existing ports, breaking their flow state.
* **The Solution:** Implemented `devx test ui`. Reads `databases:` from `devx.yaml`, boots completely isolated, ephemeral containers on random free ports with anonymous volumes, injects `DATABASE_URL` / `<ENGINE>_URL` into the test process environment, runs an optional idempotent `setup` (e.g. migrations), executes the test command, then unconditionally tears down all containers and volumes on exit. Supports full YAML configuration (`test.ui.setup`, `test.ui.command`) and CLI flag overrides (`--setup`, `--command`, `--runtime`) per the CLI + YAML parity design principle.
* **Key files:** `cmd/test.go`, `cmd/test_ui.go`, `internal/testing/ephemeral.go`

### 31. Unified OpenAPI & 3rd-Party Mocking (DONE)
* **The Problem:** If Stripe, Twilio, or an internal downstream team's API goes down, local development is completely blocked. Developers can't test integration flows without the real API being available.
* **The Solution:** Implemented `devx mock` — spins up persistent `stoplight/prism` background containers that serve schema-faithful HTTP responses from any remote OpenAPI spec. Supports full lifecycle management: `devx mock up`, `devx mock list`, `devx mock restart`, and `devx mock rm`. Environment variables (`MOCK_<NAME>_URL`) are automatically injectable. Also added `devx db restart` to close the parity gap.
* **Key files:** `cmd/mock.go`, `cmd/mock_up.go`, `cmd/mock_list.go`, `cmd/mock_restart.go`, `cmd/mock_rm.go`, `cmd/db_restart.go`, `internal/mock/server.go`

### 32. Zero-Config Local Kubernetes (Kind / k3s) (DONE)
* **The Problem:** `devx` excels at standard container execution, but developers shipping to Kubernetes ultimately need to test manifests locally without destroying their macbooks with Minikube or navigating the heavy bootstrapping of the `kind` CLI.
* **The Solution:** Implemented `devx k8s spawn`. Directly orchestrates the raw `rancher/k3s` container inside Podman/Docker to instantly boot a fully compliant Kubernetes control plane in seconds. Safely extracts the kubeconfig to an isolated scoped file (e.g. `~/.kube/devx-<name>.yaml`) ensuring no corruption to the host's primary configuration. Includes full lifecycle via `devx k8s list` and `devx k8s rm`.
* **Key files:** `cmd/k8s_spawn.go`, `cmd/k8s_list.go`, `internal/k8s/k3s.go`

### 33. CLI Integration Test Harness (DONE)
* **The Problem:** The `cmd/` layer — our most user-facing surface — had zero test coverage. Commands like `devx shell`, `devx scaffold`, and `devx cloud spawn` contain complex branching logic (env injection, idempotency guards, mount detection) that was entirely untested, creating a silent regression risk on every PR.
* **The Solution:** Built a dedicated integration test harness for the `cmd/` package. Uses a fake/mock container runtime backend to allow tests to run without a real Podman VM. Written table-driven test cases covering the most critical code paths: AI bridge injection logic, `.env` override precedence, and `--force` flag behavior on scaffold, whilst ensuring interactive TUI prompts (`huh`) don't hang automated tests.
* **Key files:** `cmd/shell_test.go`, `cmd/scaffold_test.go`, `internal/testutil/fake_runtime.go`

---

## 🟢 P0 — Build Now (Critical Foundation)

> **Recommended Sprint:** Ship 34 + 35 + 36 together. This trio transforms `devx.yaml` multi-service orchestration from "fragile demo" to "production-grade local platform."

### 34. Intelligent Service Dependency Graphs (DONE)
* **Priority:** 🟢 P0
* **Effort:** Medium
* **Impact:** Critical — table-stakes for any serious orchestration tool. Without it, multi-service `devx.yaml` topologies are fundamentally fragile. Every developer who has ever run `docker compose up` and watched 3 services crash because Postgres wasn't ready knows this pain. Undermines the credibility of `devx cloud spawn` and `devx db spawn` when used together.
* **The Problem:** When running multiple services via `devx.yaml`, services often crash on startup with "Connection Refused" because their underlying dependent services (like databases or other APIs) haven't fully initialized.
* **The Solution:** Introduce `depends_on` functionality with robust `healthcheck` gating directly inside `devx.yaml` (inspired by Compose). `devx` orchestrates the boot order, pausing the start of a frontend app until the backend API confirms readiness, eliminating crash loops.

### 35. Context-Aware "Log-Tailing" on Crash (DONE)
* **Priority:** 🟢 P0
* **Effort:** Trivial
* **Impact:** High — a 50-line feature that saves thousands of hours collectively. When a container dies during startup, dumping the last N lines inline is the obvious thing to do, and no orchestration tool does it well. Tiny effort, huge developer trust signal.
* **The Problem:** When a complex startup sequence fails, the developer only gets a generic exit code and then has to manually execute `devx logs` and scroll to find the failure, breaking context.
* **The Solution:** When `devx up` detects a container exiting prematurely, it automatically intercepts and prints the last 50 lines of the specific crashing container's logs with error highlighting directly in the terminal to immediately context-switch the developer into debugging.

### 36. Automatic Port Conflict Resolution (Port Shifting) (DONE)
* **Priority:** 🟢 P0
* **Effort:** Low
* **Impact:** High — polish that separates a tool from a product. Because `devx` already owns the entire routing chain (Cloudflare tunnel → env injection → container), it can transparently shift ports without the developer noticing. Most tools can't do this because they don't control the full stack. `devx` can. It's a genuine architectural advantage.
* **The Problem:** If a developer spins up two apps or forgets they have a ghost Node process running on port 8080, `devx` currently throws an `EADDRINUSE` failure, stopping their workflow until they hunt down and kill the process.
* **The Solution:** Auto-detect port collisions and dynamically shift to an available port (e.g., `8081`). Because `devx` controls the entire tunnel routing and environment variable injection, the ingress and local `.env` variables mapped into the app update seamlessly, acting completely transparent to the user.
* **Key files:** `internal/network/ports.go`

---

## 🟢 P1 — Build Next (High-Value Polish)

> **Recommended Sprint:** Ship 37 + 38 + 39 together as the "polish pass" after P0 lands.

### 37. Environment Overlays & Profiles (DONE)
* **Priority:** 🟢 P1
* **Effort:** Low-Medium
* **Impact:** High — essential for teams larger than 1. The moment two developers need different local topologies (frontend dev doesn't need Kafka, backend dev doesn't need the React app), you need profiles. Without them, `devx.yaml` becomes a monolith that everyone forks locally and never commits back. Skaffold nailed this; `devx` can do it better with first-class `--profile` support.
* **The Problem:** Switching between a lightweight local stack and a full integration stack involves manually commenting/uncommenting sections of `devx.yaml`, which risks polluting commits.
* **The Solution:** Add Skaffold-style `--profile` flagging. `devx.yaml` can define specific blocks (e.g., `profiles: staging`) that conditionally override the base configuration, allowing developers to swap entire environments instantly.

### 38. Native Secrets Redaction in Logs (DONE)
* **Priority:** 🟢 P1
* **Effort:** Low-Medium
* **Impact:** High (security) — a sleeper hit. The moment someone screenshares a `devx logs` session on Zoom and leaks a production API key, you'll wish you'd built this. The architectural advantage is that `devx` already knows every secret value from vault integration. Most log redaction tools guess patterns; `devx` can do exact-match replacement. That's rare and powerful.
* **The Problem:** As `devx` natively integrates Vault secrets into the environment, developers risk accidentally screensharing or screenshotting `devx logs` or `webhook catch` TUIs that output real API keys in plaintext.
* **The Solution:** Build a middleware masking engine into the Bubble Tea TUI components. `devx` already knows the exact values of secrets pulled from vaults; it can automatically redact them in all `devx logs` output and webhook payloads.
* **Key files:** `internal/logs/redactor.go`, `internal/webhook/tui.go`

### 39. Visual Architecture Map Generator (DONE)
* **Priority:** 🟢 P1
* **Effort:** Low
* **Impact:** High (onboarding) — `devx map` parsing `devx.yaml` and emitting a Mermaid diagram is ~200 lines of Go. But the impact on onboarding is outsized — a new engineer clones a repo, runs `devx map`, and instantly *sees* how 8 services connect. This is a demo-day feature. It sells the tool.
* **The Problem:** For onboarding engineers, looking at a 300-line `devx.yaml` is overwhelming and the logical flow of which app talks to what database is lost.
* **The Solution:** Implement `devx map`. It parses the internal routing, volumes, and `devx.yaml` dependencies to instantly spit out an interactive SVG or Mermaid.js graph to `<project_root>/devx-map.html`, giving visual tangibility to the stack.
