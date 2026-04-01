# What is devx?

**devx** is a developer experience CLI that provisions a fully-configured local development environment in a single command. It eliminates the three most common sources of developer friction:

| Problem | devx Solution |
|---------|---------------|
| "It works on my machine" | Deterministic Fedora CoreOS VM with pre-tuned kernel parameters |
| Accessing internal services | Zero-trust Tailscale mesh VPN, built into the VM |
| Webhooks & sharing | Instant public HTTPS via Cloudflare Tunnels (`*.ipv1337.dev`) |

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
