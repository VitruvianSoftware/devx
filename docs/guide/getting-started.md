# Getting Started

## Prerequisites

The fastest way to set up your environment is with the built-in health check:

```bash
devx doctor            # audit tools, credentials, and feature readiness
devx doctor install    # install any missing tools via your package manager
devx doctor auth       # walk through authenticating required services
```

`devx doctor` checks all CLI tools and credentials that devx uses — grouped by feature area — and tells you exactly what's ready and what needs attention.

### Manual Setup

If you prefer to install manually:

| Tool | Feature Area | Install |
|------|-------------|---------|
| [Podman](https://podman.io), [Docker](https://docker.com), [OrbStack](https://orbstack.dev), [Lima](https://lima-vm.io/), [Colima](https://github.com/abiosoft/colima) | Core VM | `brew install podman` / `docker` / `orbstack` / `lima` / `colima` |
| [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started/) | Tunnels | `brew install cloudflare/cloudflare/cloudflared` |
| [butane](https://coreos.github.io/butane/) | VM Init | `brew install butane` |
| [gh](https://cli.github.com) | Sites, Preview | `brew install gh` |
| [AWS CLI](https://aws.amazon.com/cli/) | State Replication | `brew install awscli` |
| [gcloud](https://cloud.google.com/sdk) | Vault, State Replication | `brew install google-cloud-sdk` |
| [nerdctl](https://github.com/containerd/nerdctl) | Container Runtime | `brew install nerdctl` |
| [1Password CLI](https://1password.com/downloads/command-line), [Bitwarden CLI](https://bitwarden.com/help/cli/) | Vault | `brew install 1password-cli` / `bitwarden-cli` |
| [Mutagen](https://mutagen.io) | File Sync | `brew install mutagen-io/mutagen/mutagen` |
| [kubectl](https://kubernetes.io/docs/tasks/tools/) | Bridge | `brew install kubectl` |

### Choosing a VM Backend

`devx` is VM-agnostic. It auto-detects installed backends (`podman`, `lima`, `colima`, `docker`, `orbstack`).

If multiple are found, you will be prompted to choose. You can override this behavior in three ways:
1. **Machine-local config (Recommended):** Set your preference in `~/.devx/config.yaml`.
2. **Project-local config:** Set `provider: lima` in your `devx.yaml`.
3. **CLI Flag:** Pass `--provider=colima` to any VM-dependent command.

### From Homebrew (recommended for macOS/Linux)

```bash
brew install vitruviansoftware/tap/devx
```

### From GitHub Releases

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

This creates a Fedora CoreOS VM via your chosen provider with Cloudflare Tunnel and Tailscale pre-configured. The process takes about 2-3 minutes on the first run.

### 2. Verify everything is running

```bash
devx vm status
```

You should see all three components — VM, Cloudflare Tunnel, and Tailscale — reporting as healthy.

### 3. Expose a port

```bash
# Start a web server inside the VM
devx exec <provider-runtime> run -d -p 8080:80 docker.io/nginx

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

::: tip
`devx` automatically searches upward from your current directory to find `devx.yaml`, so you can run commands like `devx up` from any subdirectory within your project — no need to `cd` back to the root.
:::

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
