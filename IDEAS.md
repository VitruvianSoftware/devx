# Future Enhancements & IDEAS

This document outlines the top 10 planned features to make `devx` the undisputed leader in local developer productivity. These ideas focus on breaking down traditional paywalls (like those found in ngrok) and making `devx` hyper-extensible for any team.

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

### 23. Local Email Catcher & Inspector
* **The Problem:** Testing transactional emails locally risks accidentally emailing real users or requires developers to sign up for external services like Mailtrap.
* **The Solution:** Add `devx mail spawn`. This spins up an in-memory SMTP server (like MailHog or Inbucket) inside the VM, exposes SMTP port 1025 to the application, and automatically generates a Cloudflare tunnel or local web UI to inspect rendered HTML emails instantly in the browser.

### 24. Outbound Webhook Catcher & Request Bin
* **The Problem:** We already solved *incoming* webhooks via Cloudflare Tunnels, but when the local app needs to *send* a webhook payload to a 3rd party, inspecting exactly what headers and JSON was sent often requires setting up a remote RequestBin in the browser.
* **The Solution:** Add `devx webhook catch`. This spawns a local server acting as a dummy endpoint, injects `WEBHOOK_URL=http://...` into the environment, and displays a beautiful terminal UI logging every outgoing HTTP request the application attempts to fire, complete with payload formatting.

### 25. Secure Production Data Anonymization & Pull
* **The Problem:** `devx db spawn` offers a blank database, which is useless for fixing a bug that only occurs with specific production data shapes. Dumping production data locally is a massive security/compliance risk.
* **The Solution:** Add `devx db pull <env>`. This integrates with cloud platforms (like custom GCP/AWS scripts) to trigger an automated, ephemeral staging job that dumps the requested database, runs a configured anonymization script (masking PII, emails, passwords), downloads the sanitized dump via the Tailnet, and streams it directly into the local `devx db spawn` instance.

### 26. Instant Vulnerability & Secret Scanning
* **The Problem:** Developers accidentally commit `.env` files or introduce NPM/Go vulnerabilities, which are only caught much later by GitHub Actions CI, breaking the build and requiring a context-switching PR fix.
* **The Solution:** Add `devx audit`. Before pushing code, developers can run this command to spin up an ephemeral container equipped with tools like Trivy or Gitleaks. It mounts the current directory read-only, performs a blisteringly fast local scan for leaked secrets or CVEs, and prevents the "CI walk of shame."
