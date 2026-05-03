# What is devx?

## Mission Statement

**devx** exists to bring absolute joy back to local development. 
We relentlessly eliminate the daily friction that pulls developers out of their flow state. From wrestling with inconsistent OS kernels to manually managing `.env` files, mocking webhooks, or resetting scrambled testing databases—we believe your tooling should work natively, instantly, and invisibly so you can just write code.

## Why `devx`? (More than just Compose or Skaffold)

While tools like **Docker Compose** excel at booting containers and **Skaffold** focuses on bridging local workflows to Kubernetes, `devx` serves as a comprehensive, end-to-end **Local Development Environment Orchestrator**. 

We go far beyond basic container networking by natively integrating the premium capabilities developers usually pay for (or duct-tape together) into a single, unified CLI. Crucially, `devx` seamlessly bridges the gap between **local standalone containers** and **remote Kubernetes clusters** within the exact same workflow:

| Problem | devx Solution |
|---------|---------------|
| "It works on my machine" | Deterministic Fedora CoreOS VM with pre-tuned kernel parameters |
| Accessing internal services | Zero-trust Tailscale mesh VPN silently built into the VM |
| Costly ngrok subscriptions | Instant public HTTPS via Cloudflare Tunnels (`*.ipv1337.dev`) |
| Broken UI tests corrupting DBs | Isolated, dynamically allocated Ephemeral E2E Browser Testing Databases |
| Outdated local `.env` files | Native injection of secrets directly from Bitwarden and 1Password |
| Juggling multiple CLI tools | Integrated Bubble Tea TUIs for log multiplexing and webhook caching |


## Configuration Domains

The `devx` ecosystem separates configuration into two distinct files based on the scope of orchestration:

1. **`devx.yaml` (Project-Level Local & Hybrid Dev):** This is the primary configuration file. It lives in your application's repository and defines your topology—this includes local standalone containers (databases, APIs) as well as **Hybrid Bridge connections** that route traffic directly to and from remote Kubernetes clusters. It is used by almost all `devx` commands (e.g., `devx up`, `devx test`, `devx action`).
   - **Discovery Behavior**: `devx` automatically searches the current directory and all parent directories upward until it finds a `devx.yaml` file. This allows you to seamlessly run `devx` commands from any nested subdirectory within your project.
2. **`cluster.yaml` (Infrastructure-Level Multi-Node Dev):** This file is exclusively used by the `devx cluster` command suite. It defines the desired state of a multi-node Kubernetes cluster (node IPs, K3s versions, VM allocations) and is usually kept in a dedicated infrastructure repository.
   - **Discovery Behavior**: Similar to `devx.yaml`, `devx cluster` automatically crawls upward from the current directory to locate your `cluster.yaml` configuration.

These files do not override each other; they serve completely different domains.

### Discovery Order (`devx.yaml`)

1. Walk upward from CWD → first `devx.yaml` found wins
2. `include:` directives within that file compose additional configs (relative to the config's directory)
3. `~/.devx/config.yaml` provides machine-local overrides (e.g., VM provider)

## How It Works

```bash
devx vm init    # One command. Done.
```

Behind the scenes, `devx` provisions a **Podman Machine** running Fedora CoreOS, then injects an [Ignition](https://coreos.github.io/butane/) config that:

1. **Installs Tailscale** and joins your corporate Tailnet automatically
2. **Creates a Cloudflare Tunnel** with persistent credentials
3. **Tunes the kernel** — sets `inotify` limits, `fs.aio-max-nr`, and other parameters
4. **Exposes ports** through Cloudflare so external services (Stripe, GitHub webhooks, teammates) can reach your local machine

## Who Is It For?

- **Application developers** who need a consistent, pre-configured container runtime
- **Platform engineers** who want to standardize the dev environment across a team
- **Open-source maintainers** who want to reduce onboarding friction to near-zero

## Design Principles

- **One CLI, everything** — VM, tunnels, databases, agent skills, and site hosting are all subcommands of `devx`
- **Convention over configuration** — Sensible defaults (`devx vm init` works with zero flags), but everything is overridable
- **Transparency** — Destructive operations show an impact summary and require confirmation
- **Idempotency** — Commands are designed to be run repeatedly safely. Existing configurations and files are skipped or patched contextually to preserve developer intent.
- **AI-native** — Agent skill files and `--json` output make `devx` controllable by AI coding assistants
- **CLI + YAML parity** — Every configurable behavior is available both as a CLI flag (for one-off use and scripting) and as a `devx.yaml` property (for committed team defaults). Neither mode is a second-class citizen.
- **Optimized Inner Loop** — Developer flow state is sacred. Every feature, from sub-millisecond Cloudflare ingress to instant ephemeral database testing, is optimized to radically reduce the "code-to-feedback" cycle time.
- **Client-Side First Architecture** — No bloated centralized SaaS proxy servers or massive Kubernetes cluster controllers required. `devx` runs completely locally, orchestrating standard daemons (Tailscale, Cloudflared, Podman) natively on your host.
- **Absolute Portability** — "It works on my machine" is solved permanently. Because `devx` standardizes a Fedora CoreOS Podman Machine locally, your testing and execution topology is indistinguishable regardless of your host OS (Mac/Windows/Linux) or processor architecture.
- **Future-Proofing for Growth** — Advanced features like predictive background pre-building and telemetry nudges are opt-in. Small projects stay lightweight, while scaling teams unlock powerful optimizations exactly when their workflows demand it — no premature complexity.
- **Familiarity-First** — Developers who have used Skaffold or Docker Compose should feel at home. `devx` combines the service orchestration of Compose with the pipeline stages of Skaffold, then adds capabilities neither tool offers — local observability, predictive pre-building, secret vault injection, and agentic shipping.
