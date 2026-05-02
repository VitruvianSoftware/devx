# Getting Started

## Prerequisites

The fastest way to set up your environment is with the built-in health check:

```bash
devx doctor            # audit tools, credentials, and feature readiness
devx doctor install    # install any missing tools via your package manager
devx doctor auth       # walk through authenticating required services
```

`devx doctor` checks for all 9 CLI tools and 7 credentials that devx uses, and tells you exactly what's ready and what needs attention.

### Manual Setup

If you prefer to install manually:

| Tool | Install | Purpose | Required? |
|------|---------|---------| --------- |
| [Podman](https://podman.io) | `brew install podman` | VM and container runtime | Yes |
| [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started/) | `brew install cloudflare/cloudflare/cloudflared` | Cloudflare tunnel daemon | Yes |
| [butane](https://coreos.github.io/butane/) | `brew install butane` | Ignition config compiler | Yes |
| [gh](https://cli.github.com) | `brew install gh` | GitHub CLI (for `devx sites`) | Yes |
| [docker](https://docker.com) | `brew install docker` | Alternative VM backend | Optional |
| [orbstack](https://orbstack.dev) | `brew install orbstack` | Alternative VM backend | Optional |
| [1Password CLI](https://1password.com/downloads/command-line) | `brew install 1password-cli` | Vault secret sync | Optional |
| [Bitwarden CLI](https://bitwarden.com/help/cli/) | `brew install bitwarden-cli` | Vault secret sync | Optional |
| [gcloud](https://cloud.google.com/sdk) | `brew install google-cloud-sdk` | GCP Secret Manager | Optional |

## Installation

### From Homebrew (macOS/Linux)

```bash
brew install vitruviansoftware/tap/devx
```

### From GitHub Releases (recommended)

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

## Quick Start

### 0. Check your environment (one-time)

```bash
devx doctor
```

This audits all prerequisites — tools, credentials, and authentication sessions — and tells you exactly what's ready and what needs fixing. If anything is missing, follow the prompts or run `devx doctor install`.

### 1. Provision your dev environment

```bash
devx vm init
```

This creates a Fedora CoreOS VM via Podman Machine with Cloudflare Tunnel and Tailscale pre-configured. The process takes about 2-3 minutes on the first run.

### 2. Verify everything is running

```bash
devx vm status
```

You should see all three components — VM, Cloudflare Tunnel, and Tailscale — reporting as healthy.

### 3. Expose a port

```bash
# Start a web server inside the VM
devx exec podman run -d -p 8080:80 docker.io/nginx

# Expose it to the internet
devx tunnel expose 8080 --name demo
# → https://demo.your-name.ipv1337.dev
```

### 4. Spin up a database

```bash
devx db spawn postgres --name mydb
# → PostgreSQL running on localhost:5432
```

## Configuration

`devx` uses a `.env` file in your project root for secrets:

```bash
# .env
CF_API_TOKEN=your-cloudflare-api-token
CF_TUNNEL_TOKEN=your-tunnel-token
DEV_HOSTNAME=your-machine-name
```

::: tip
Run `devx doctor auth` to set up your `.env` interactively — it will prompt for tokens and save them automatically.
:::

### Validating Your Environment Variables

Before starting your app, use `devx config validate` to catch missing or empty secrets early — before they cause cryptic runtime crashes:

```bash
devx config validate
```

```
📋 Schema: .env.example
🔑 Secret source: devx.yaml (op://my-vault/myapp/...)

  ✓ CF_API_TOKEN
  ✓ CF_TUNNEL_TOKEN
  ✗ STRIPE_SECRET_KEY  (missing — not found in any vault source)
  ⚠ OPENAI_API_KEY     (present but empty)

  2 of 4 keys failed validation

  Run 'devx config pull' to sync secrets from your vault.
```

`devx config validate` reads the required keys from `.env.example` (or `.env.schema`), fetches the actual values from your configured vault sources in `devx.yaml`, and reports any gaps. Use `--json` for CI pipelines or AI agents.

Global flags available on all commands:

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview destructive actions without executing |
| `--json` | Machine-readable output for AI agents |
| `-y, --non-interactive` | Skip confirmation prompts |
| `--env-file` | Path to secrets file (default: `.env`) |
