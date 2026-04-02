---
name: devx-orchestrator
description: Defines the standard operating procedures and CLI flags for AI agents interacting with the Devx virtualized local development environment.
---

# Devx AI Agent Guidelines

You are operating in a repository that uses `devx`, a Go-based CLI tool orchestrating Podman, Cloudflared, and Tailscale for local development.

When you need to interact with the local infrastructure, databases, or environment networking, ALWAYS use `devx` CLI commands rather than manually writing shell scripts for docker/podman or cloudflared.

## 🤖 1. Machine-Readable Context (`--json`)

Never try to parse the human-readable TUI output of devx status commands using Regular Expressions or text slitting. Devx has full support for strictly deterministic structural state via the `--json` flag.

Always append `--json` when you are querying the environment state:
- `devx vm status --json`: Returns a JSON object with VM health, Tailscale auth state, and Cloudflare domains.
- `devx db list --json`: Returns a JSON array of running PostgreSQL/MySQL/Redis engines and their exposed ports.
- `devx tunnel list --json`: Returns a JSON array summarizing active localhost internet exposures.

## 🛑 2. Non-interactive Execution (`--non-interactive` / `-y`)

Many devx commands invoke interactive terminal surveys (via `charmbracelet/huh`) to ask the human developer for confirmation before acting. As an AI Agent, you lack a TTY to press 'Enter', which will cause you to stall indefinitely.

**You must ALWAYS use the `--non-interactive` (or `-y`) flag on mutating commands:**
- `devx vm teardown -y` (Will skip the deletion confirm warning and execute instantly)
- `devx db rm postgres -y` (Will skip data deletion warnings and execute instantly)
- `devx init -y` (Will hard-fail immediately with an exit error if required credentials are not in `.env`, rather than freezing to ask for them)

## 🦺 3. Safe Preflight Testing (`--dry-run`)

If you are asked to clean up the environment, but you are not 100% confident in the scope of the destruction, use the `--dry-run` flag.

- `devx vm teardown --dry-run`
- `devx db rm <engine> --dry-run`
- `devx tunnel unexpose --dry-run`

Devx will intercept the execution path and safely echo out precisely which containers, internet URLs, and persistent data volumes *would* be destroyed, allowing you to ask the human for approval.

## 🚦 4. Deterministic Exit Codes

When running a command that fails, `devx` avoids polluting standard error with useless `--help` output. Instead, it utilizes predictive numeric Exit Codes to signal exactly what went wrong so you can programmatically trap and rescue the state cleanly:

- `Exit 15 (CodeVMDormant)`: The VM exists but is sleeping. It could not automatically wake up.
- `Exit 16 (CodeVMNotFound)`: The VM has been deleted. You must run `devx vm init`.
- `Exit 22 (CodeHostPortInUse)`: You attempted to run `devx db spawn <engine>`, but the host port is already allocated by another daemon. Try a different port using `-p <port>`.
- `Exit 41 (CodeNotLoggedIn)`: You attempted to expose a tunnel, but `cloudflared` is not authenticated on this machine. Request that the user run `cloudflared tunnel login`.
