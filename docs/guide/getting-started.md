# Getting Started

## Prerequisites

Install these tools before running `devx`:

| Tool | Install | Purpose |
|------|---------|---------|
| [Podman](https://podman.io) | `brew install podman` | VM and container runtime |
| [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started/) | `brew install cloudflare/cloudflare/cloudflared` | Cloudflare tunnel daemon |
| [butane](https://coreos.github.io/butane/) | `brew install butane` | Ignition config compiler |

## Installation

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

### 1. Authenticate with Cloudflare (one-time)

```bash
cloudflared login
```

This opens a browser to authorize `cloudflared` against your Cloudflare account. The generated certificate is stored at `~/.cloudflared/cert.pem`.

### 2. Provision your dev environment

```bash
devx vm init
```

This creates a Fedora CoreOS VM via Podman Machine with Cloudflare Tunnel and Tailscale pre-configured. The process takes about 2-3 minutes on the first run.

### 3. Verify everything is running

```bash
devx vm status
```

You should see all three components — VM, Cloudflare Tunnel, and Tailscale — reporting as healthy.

### 4. Expose a port

```bash
# Start a web server inside the VM
devx exec podman run -d -p 8080:80 docker.io/nginx

# Expose it to the internet
devx tunnel expose 8080 --name demo
# → https://demo.your-name.ipv1337.dev
```

### 5. Spin up a database

```bash
devx db spawn postgres --name mydb
# → PostgreSQL running on localhost:5432
```

## Configuration

`devx` uses a `.env` file in your project root for secrets:

```bash
# .env
CLOUDFLARE_API_TOKEN=your-token-here
TAILSCALE_AUTH_KEY=tskey-auth-...
```

Global flags available on all commands:

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview destructive actions without executing |
| `--json` | Machine-readable output for AI agents |
| `-y, --non-interactive` | Skip confirmation prompts |
| `--env-file` | Path to secrets file (default: `.env`) |
