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

Run the built-in health check to audit and install prerequisites automatically:

```bash
devx doctor            # check what's installed
devx doctor install    # install missing tools
devx doctor auth       # authenticate required services
```

Or install them manually:

| Tool | Install | Purpose |
|------|---------|---------|
| [Podman](https://podman.io) | `brew install podman` | VM and container runtime |
| [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started/) | `brew install cloudflare/cloudflare/cloudflared` | Cloudflare tunnel daemon |
| [butane](https://coreos.github.io/butane/) | `brew install butane` | Ignition config compiler |
| [gh](https://cli.github.com) | `brew install gh` | GitHub CLI (for `devx sites`) |

## Quick Start

```bash
# 0. Check your environment (one-time)
devx doctor

# 1. Provision your dev environment
devx vm init

# 2. Run something and expose it
devx exec podman run -d -p 8080:80 docker.io/nginx
# Visit https://your-name.ipv1337.dev — it's live!

# 3. Expose any local port instantly (like ngrok)
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

## 📚 Documentation

The full documentation for `devx`, including all CLI commands, advanced networking, and AI Agent workflows, is available at [devx.vitruviansoftware.dev](https://devx.vitruviansoftware.dev).

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) for details on:

- Development setup
- Code style and conventions
- Pull request process
- Commit message format

## License

[MIT](LICENSE) © VitruvianSoftware