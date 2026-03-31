<p align="center">
  <h1 align="center">⚡ devx</h1>
  <p align="center">Supercharged local dev environment — Podman + Cloudflare Tunnels + Tailscale in one CLI</p>
  <p align="center">
    <a href="https://github.com/VitruvianSoftware/devx/actions/workflows/ci.yml"><img src="https://github.com/VitruvianSoftware/devx/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://github.com/VitruvianSoftware/devx/releases/latest"><img src="https://img.shields.io/github/v/release/VitruvianSoftware/devx" alt="Release"></a>
    <a href="https://github.com/VitruvianSoftware/devx/blob/main/LICENSE"><img src="https://img.shields.io/github/license/VitruvianSoftware/devx" alt="License"></a>
    <a href="https://goreportcard.com/report/github.com/VitruvianSoftware/devx"><img src="https://goreportcard.com/badge/github.com/VitruvianSoftware/devx" alt="Go Report Card"></a>
  </p>
</p>

<p align="center">
  <img src=".github/assets/hero.png" alt="devx — Supercharged Local Development" width="600">
</p>

---

`devx` provisions a customized **Fedora CoreOS** VM via Podman Machine and automatically configures **Cloudflare Tunnels** (instant public HTTPS) and **Tailscale** (zero-trust corporate network access) — all from a single command.

## The Problem

Local development is plagued by recurring friction:

1. **"It works on my machine"** — Inconsistent host OS configs, file watcher limits, kernel parameters
2. **Accessing internal services** — Developers need corporate APIs/databases without routing everything through a slow VPN
3. **Webhooks & sharing** — Testing Stripe/GitHub webhooks or sharing a prototype requires sketchy ngrok setups

## The Solution

```bash
devx vm init    # One command. Done.
```

You get a fully-configured Fedora CoreOS VM with:

- 🌐 **Instant public HTTPS** — Your machine gets `your-name.ipv1337.dev` automatically
- 🔒 **Zero-trust corporate access** — The VM joins your Tailnet transparently
- 🚀 **ngrok-like port exposure** — `devx tunnel expose 3000` gives you a public URL in seconds
- 🏗️ **Host-level isolation** — Pre-tuned `inotify` limits, rootful containers, dedicated kernel

## Installation

### From Releases (recommended)

Download the latest binary from [GitHub Releases](https://github.com/VitruvianSoftware/devx/releases/latest):

```bash
# macOS (Apple Silicon)
curl -sL https://github.com/VitruvianSoftware/devx/releases/latest/download/devx_darwin_arm64.tar.gz | tar xz
sudo mv devx /usr/local/bin/

# macOS (Intel)
curl -sL https://github.com/VitruvianSoftware/devx/releases/latest/download/devx_darwin_amd64.tar.gz | tar xz
sudo mv devx /usr/local/bin/

# Linux (amd64)
curl -sL https://github.com/VitruvianSoftware/devx/releases/latest/download/devx_linux_amd64.tar.gz | tar xz
sudo mv devx /usr/local/bin/
```

### From Source

```bash
go install github.com/VitruvianSoftware/devx@latest
```

### Prerequisites

These tools must be installed before running `devx vm init`:

| Tool | Install | Purpose |
|------|---------|---------|
| [Podman](https://podman.io) | `brew install podman` | VM and container runtime |
| [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started/) | `brew install cloudflare/cloudflare/cloudflared` | Cloudflare tunnel daemon |
| [butane](https://coreos.github.io/butane/) | `brew install butane` | Ignition config compiler |

## Quick Start

```bash
# 1. Authenticate with Cloudflare (one-time)
cloudflared login

# 2. Provision your dev environment
devx vm init

# 3. Run something and expose it
devx exec podman run -d -p 8080:80 docker.io/nginx
# Visit https://your-name.ipv1337.dev — it's live!

# 4. Expose any local port instantly (like ngrok)
devx tunnel expose 3000 --name myapp
# → https://myapp.your-name.ipv1337.dev
```

## Architecture

```mermaid
flowchart TB
    subgraph devlaptop["Developer's Mac"]
        direction TB
        Code[VS Code / Terminal] --> |podman run| UserContainers

        subgraph podmanvm["Podman Machine (Fedora CoreOS)"]
            subgraph daemons["Systemd Controlled"]
                TS[Tailscale Daemon]
                CF[Cloudflared Tunnel]
            end

            subgraph UserContainers["Developer's Apps"]
                App["API / Web App<br/>Port 8080"]
                DB[(Local DB)]
            end

            CF -->|Forwards Ingress| App
            TS -->|Exposes Subnets| App
        end
    end

    subgraph internet["Public Web"]
        CFEdge((Cloudflare Edge))
        PublicURL["https://developer.ipv1337.dev"]
        ExternalUser((External User / Webhook))

        ExternalUser --> PublicURL
        PublicURL --> CFEdge
    end

    subgraph tailnet["Internal Network (Corporate Tailnet)"]
        StagingDB[(Staging Database)]
        InternalAPI[Internal Microservices]
    end

    CFEdge <-->|Secure Encrypted Tunnel| CF
    TS <-->|Zero-Trust VPN Overlay| tailnet

    classDef vm fill:#f0f4f8,stroke:#0288d1,stroke-width:2px;
    classDef daemon fill:#e1f5fe,stroke:#0277bd,stroke-width:1px;
    classDef container fill:#e8f5e9,stroke:#2e7d32,stroke-width:1px;

    class podmanvm vm;
    class TS,CF daemon;
    class App,DB container;
```

## CLI Reference

```
devx — Supercharged local dev environment

Commands:
  vm          Manage the local development VM
  tunnel      Manage Cloudflare tunnels and port exposure
  shell       Launch a dev container from devcontainer.json
  db          Manage local development databases
  config      Manage devx configuration and credentials
  exec        Run low-level infrastructure tools directly
  version     Print the devx version
```

### 🗄️ Database Provisioning (`devx db`)

Spin up local databases in one command with persistent volumes. No Docker Compose files, no YAML — just `devx db spawn`.

| Command | Description |
|---------|-------------|
| `devx db spawn <engine>` | Start a database with persistent storage |
| `devx db list` | List all devx-managed databases |
| `devx db rm <engine>` | Stop and remove a database (prompts for confirmation) |

**Supported engines:**

| Engine | Image | Default Port |
|--------|-------|-------------|
| `postgres` | `postgres:16-alpine` | 5432 |
| `redis` | `redis:7-alpine` | 6379 |
| `mysql` | `mysql:8` | 3306 |
| `mongo` | `mongo:7` | 27017 |

```bash
# Spawn a PostgreSQL database — ready in seconds
devx db spawn postgres
# → postgresql://devx:devx@localhost:5432/devx

# Spawn Redis on a custom port
devx db spawn redis --port 6380

# Spawn MySQL using Docker instead of Podman
devx db spawn mysql --runtime docker

# List all running databases
devx db list

# Remove a database but keep its data volume for later
devx db rm postgres --keep-volume

# Fully remove a database and its data
devx db rm mongo
```

Data is stored in named volumes (`devx-data-<engine>`) so it **survives container restarts and rebuilds**. Containers are configured with `--restart unless-stopped` for automatic recovery.

### 🐳 Dev Containers (`devx shell`)

Drop into a fully isolated development environment powered by your project's `devcontainer.json` — no VS Code required.

`devx shell` reads your existing `devcontainer.json`, pulls the specified container image, mounts your local workspace, applies environment variables, forwards ports, runs post-create commands, and drops you into an interactive shell.

**Supported `devcontainer.json` locations:**
- `.devcontainer/devcontainer.json` (standard)
- `.devcontainer.json` (root level)
- `.devcontainer/<name>/devcontainer.json` (named configs)

```bash
# Launch a dev shell using your project's devcontainer.json
devx shell

# Use Docker instead of Podman as the container runtime
devx shell --runtime docker
```

**Example `devcontainer.json`:**
```json
{
  "name": "Go Dev Environment",
  "image": "mcr.microsoft.com/devcontainers/go:1.22",
  "remoteUser": "vscode",
  "workspaceFolder": "/workspace",
  "containerEnv": {
    "GOPATH": "/home/vscode/go"
  },
  "postCreateCommand": "go mod tidy",
  "forwardPorts": [8080, 3000]
}
```

Running `devx shell` in a project with the above config will:
1. Pull the Go 1.22 dev container image
2. Mount your project at `/workspace`
3. **Auto-mount the `devx` binary** into the container at `/usr/local/bin/devx`
4. **Mount your `~/.cloudflared` credentials** so tunnel commands work inside the container
5. **Share the host network** so exposed tunnels are accessible
6. Set `GOPATH` inside the container
7. Expose ports 8080 and 3000 on your host
8. Run `go mod tidy` on first launch
9. Drop you into an interactive shell as the `vscode` user

This means you can run `devx tunnel expose 8080` **from inside the dev container** — the full CLI travels with you.

### VM Management (`devx vm`)

| Command | Description |
|---------|-------------|
| `devx vm init` | Full first-time provisioning with interactive TUI |
| `devx vm init --provider docker` | Provision using Docker Desktop instead of Podman |
| `devx vm init --provider orbstack` | Provision using OrbStack |
| `devx vm status` | Show health of VM, Cloudflare tunnel, and Tailscale |
| `devx vm teardown` | Stop and permanently remove the VM (prompts for confirmation) |
| `devx vm ssh` | Drop into an SSH shell inside the VM |

### 🔌 Backend Pluggability (`--provider`)

`devx` is not locked to Podman. Use whatever virtualization backend you prefer by passing the `--provider` flag on any `devx vm` subcommand.

**Supported providers:**

| Provider | CLI Value | Description |
|----------|-----------|-------------|
| Podman Machine | `podman` (default) | Fedora CoreOS + rootful containers |
| Docker Desktop | `docker` | Uses `docker` CLI; auto-detects Docker Desktop VM |
| OrbStack | `orbstack` | Lightweight macOS-native Docker using `orb` CLI |

```bash
# Initialize with Docker Desktop
devx vm init --provider docker

# Check status using OrbStack
devx vm status --provider orbstack

# SSH into the OrbStack VM
devx vm ssh --provider orbstack

# Teardown a Docker-backed environment
devx vm teardown --provider docker
```

### Tunnel & Port Exposure (`devx tunnel`)

| Command | Description |
|---------|-------------|
| `devx tunnel expose [port]` | Expose a local port to the internet via `*.ipv1337.dev` |
| `devx tunnel expose [port] --name myapp` | Use a static subdomain (`myapp.you.ipv1337.dev`) |
| `devx tunnel expose [port] --basic-auth "user:pass"` | Protect your exposed URL with Basic Authentication |
| `devx tunnel inspect [port]` | Live TUI to inspect and replay HTTP traffic (like ngrok inspect) |
| `devx tunnel up` | Expose multiple mapping routes via a `devx.yaml` topology |
| `devx tunnel list` | List all active port exposures with URLs and ports |
| `devx tunnel unexpose` | Clean up all exposed tunnels |
| `devx tunnel update` | Rotate Cloudflare credentials without rebuilding the VM |

### 🗂️ Multi-Port Mapping (`devx.yaml`)

Manage complex projects that require multiple services (like a backend API, a frontend web app, and a webhook consumer) simultaneously via a unified single file.

Just create a `devx.yaml` in your project root:
```yaml
name: my-project
tunnels:
  - name: api
    port: 8080
    basic_auth: "admin:pass"
  - name: web
    port: 3000
```
Then run **`devx tunnel up`**. Your local services will automatically map under contiguous domains like `api-my-project-you.ipv1337.dev` via a singular, highly efficient Cloudflare connection.

### 🔒 Built-in Authentication (`--basic-auth`)

Exposing local ports to the public internet securely shouldn't require premium subscriptions to external services. `devx` comes with a highly-performant built-in reverse proxy in Go.

Simply pass the `--basic-auth` flag to instantly protect your active development environment with encrypted basic authentication at the edge proxy layer.

```bash
# Expose your Next.js app but require credentials
devx tunnel expose 3000 --basic-auth "admin:supersecret"

# Inspect webhooks while restricting access
devx tunnel inspect 8080 --name stripe-test --basic-auth "webhook:testing123"
```

### 🌍 Custom Domain Support (BYOD)

Want to use your own company domain instead of the free `.ipv1337.dev` sandbox? Any `devx tunnel` command supports the `--domain` override!

As long as you are logged into a Cloudflare account that owns the zone, Cloudflared will dynamically configure the requested DNS edge dynamically.

```bash
# Provision a branded tunnel endpoint on your own custom domain
devx tunnel expose 8000 --domain mycompany.dev --name api
# → https://api.mycompany.dev

# Or use it globally across an entire YAML topology
devx tunnel up --domain mycompany.dev
```

### Configuration (`devx config`)

| Command | Description |
|---------|-------------|
| `devx config secrets` | Interactive credential setup / rotation for `.env` |

### 🔍 Request Inspector (`devx tunnel inspect`)

A free, open-source replacement for ngrok's paid web inspector. Captures every HTTP request and response flowing through your tunnel in a live terminal UI.

```bash
# Inspect traffic to a local app (local-only, no tunnel)
devx tunnel inspect 8080

# Inspect AND expose via Cloudflare tunnel
devx tunnel inspect 3000 --expose

# With a static subdomain
devx tunnel inspect 8080 --name myapi
```

**Features:**
- 📋 Live scrollable request list with method, path, status, and duration
- 🔎 Detailed view showing full request/response headers and bodies
- 🔁 One-key replay (`r`) to resend any captured request
- 🏷️ Replay tagging so you can compare original vs replayed responses
- 🧹 Clear captured requests with `c`

### Configuration (`devx config`)

| Command | Description |
|---------|-------------|
| `devx config secrets` | Interactive credential setup / rotation for `.env` |

### Low-Level Tools (`devx exec`)

Pass-through wrappers that forward arguments directly to the underlying tools:

| Command | Description |
|---------|-------------|
| `devx exec podman [args]` | Run Podman commands against the VM |
| `devx exec tailscale [args]` | Interact with the VM's Tailscale daemon |
| `devx exec cloudflared [args]` | Run cloudflared commands directly |
| `devx exec butane [args]` | Run butane commands directly |

## Configuration

devx reads its secrets from a `.env` file in the current directory. Copy the example and fill in your values:

```bash
cp .env.example .env
# Edit .env with your Cloudflare tunnel token and hostname
```

See `.env.example` for all available configuration options.

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) for details on:

- Development setup
- Code style and conventions
- Pull request process
- Commit message format

## License

[MIT](LICENSE) © VitruvianSoftware