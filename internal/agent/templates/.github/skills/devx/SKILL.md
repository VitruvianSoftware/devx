---
name: devx-orchestrator
description: "Agent Skill: Use when managing the devx local environment including the Container VM, Database spawning, Cloudflare Tunnel Exposing, or networking management."
---
# GitHub Copilot Agent Skill: Devx Orchestration

You act as a co-developer within a repository orchestrated by the `devx` environment CLI (managing Podman, Cloudflare Tunnels, Tailscale).

## Core Directives for Agent Execution
1. **Interactive UI Avoidance**: Many `devx` commands use interactive forms (like `huh`) to prompt the user. You MUST append `--non-interactive` (or `-y`) to bypass them:
   - `devx vm teardown -y`
   - `devx db rm <engine> -y`

2. **JSON Output Formats**: You cannot parse the terminal TUI style outputs of Devx natively. Devx supports deterministic machine-readable statuses:
   - Run `devx vm status --json`
   - Run `devx db list --json`
   - Run `devx tunnel list --json`

3. **Dry-Run Validations**: When creating terminal sessions to destroy databases or tunnels, use `--dry-run` to output exactly what actions the tool will take without committing them.

4. **Exit Codes**: Devx uses programmatic exit codes for errors. Support fallback strategies via catching specific integers:
   - `Exit 15`: `CodeVMDormant` (VM is merely sleeping, auto-resume failed).
   - `Exit 16`: `CodeVMNotFound` (VM is destroyed. Run `devx init`).
   - `Exit 22`: `CodeHostPortInUse` (Database port is already taken on localhost).
   - `Exit 41`: `CodeNotLoggedIn` (Cloudflared is unauthenticated).
