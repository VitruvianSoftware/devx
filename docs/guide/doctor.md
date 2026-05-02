# Environment Doctor

`devx doctor` is a built-in environment health check that audits, installs, and configures all prerequisites. New developers should run it before anything else.

## Quick Overview

```bash
devx doctor            # full audit — tools, credentials, feature readiness
devx doctor install    # install missing tools
devx doctor auth       # guided credential setup
```

## `devx doctor`

Runs a full environment audit with four sections:

### System Info
Detects your OS, architecture, and package manager.

### CLI Tools
Checks all 9 tools that devx depends on, grouped by required vs optional:

| Tool | Binary | Feature | Required? |
|------|--------|---------|-----------|
| Any VM backend | `lima`, `colima`, `podman`, etc. | Core VM | Yes |
| Cloudflared | `cloudflared` | Tunnels | Yes |
| Butane | `butane` | VM Init | Yes |
| GitHub CLI | `gh` | Sites | Yes |
| Docker | `docker` | Core VM (alt) | Optional |
| OrbStack | `orb` | Core VM (alt) | Optional |
| 1Password CLI | `op` | Vault | Optional |
| Bitwarden CLI | `bw` | Vault | Optional |
| Google Cloud SDK | `gcloud` | Vault | Optional |

### Credentials
Verifies all authentication sessions and API tokens:

- **Cloudflare API Token** — checks `.env` for `CLOUDFLARE_API_TOKEN` or `CF_API_TOKEN`
- **cloudflared login** — checks for `~/.cloudflared/cert.pem`
- **GitHub CLI** — runs `gh auth status` and checks for `admin:org` scope
- **Tailscale** — detects if a VM exists (Tailscale is configured inside it during `vm init`)
- **CF Tunnel Token** — checks `.env` for `CF_TUNNEL_TOKEN`
- **Vault credentials** — checks `op`, `bw`, or `gcloud` auth status (only if installed)

### Feature Readiness
Maps tool + credential requirements to devx commands, telling you exactly which features are operational:

```
✓  devx vm init          ready
✓  devx tunnel expose    ready
✓  devx sites init       ready
✓  devx db spawn         ready
⚠️  devx config pull      needs: op, bw, or gcloud
```

## `devx doctor install`

Detects missing tools and installs them using your system's package manager.

```bash
devx doctor install          # install missing required tools only
devx doctor install --all    # include optional tools too
devx doctor install -y       # auto-confirm (no prompts)
```

The command shows you the exact install plan before executing:

```
📦 Install Plan
  Package Manager:  brew

    →  Butane                required
       brew install butane
    →  GitHub CLI            required
       brew install gh

  Install 2 tool(s)? [y/N]
```

### Supported Package Managers

| OS | Package Manager |
|----|-----------------|
| macOS | Homebrew (`brew`) |
| Linux | `apt`, `dnf`, `pacman`, `yum`, `apk`, `nix` |

## `devx doctor auth`

Walks through authenticating each required service interactively. Steps that are already configured are automatically skipped.

```bash
devx doctor auth
```

```
🔑 devx doctor auth — Credential Setup

  [1/3]  cloudflared login  ✅ ~/.cloudflared/cert.pem
  [2/3]  GitHub CLI  ✅ authenticated (admin:org ✓)
  [3/3]  Cloudflare API Token  ❌ not found
         Prompts for token and saves to .env

    Create an API token at: https://dash.cloudflare.com/profile/api-tokens
    Required permissions: Zone:DNS:Edit, Zone:Zone:Read

    Paste your Cloudflare API Token: _
```

### Auth Steps

| Step | What It Does | When Needed |
|------|-------------|-------------|
| `cloudflared login` | Opens browser to authenticate with Cloudflare | `vm init`, tunnel creation |
| `gh auth login` | Authenticates GitHub CLI with `admin:org` scope | `sites init/status` |
| Cloudflare API Token | Prompts for token and saves to `.env` | `sites init`, DNS operations |

## JSON Output

All doctor commands support `--json` for AI agent consumption:

```bash
devx doctor --json              # full audit report
devx doctor install --json      # install plan (without executing)
devx doctor auth --json         # auth step status
```
