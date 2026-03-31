---
description: "Agent Skill: Use when managing the devx local environment including the Container VM, Database spawning, Cloudflare Tunnel Exposing, or networking management."
argument-hint: Ask the user to clarify what devx component they are seeking to manage if unclear.
---
# Cursor AI Rules for Devx

This repository orchestrates `devx`, a local VM environment manager covering Podman, Cloudflare Tunnels, and Tailscale.

When interacting with tasks related to environment provisioning, database spawning, or network exposure, DO NOT write shell scripts. Instead, use the `devx` CLI interface natively provided in this project.

## Agent Restrictions
1. **Interactive Prompts**: You do not have a TTY interface. If you run a devx command that asks a question using the `huh` library or `survey`, it will block your execution forever. You MUST append `--non-interactive` (or `-y`) to mutating commands:
   - `devx init -y`
   - `devx vm teardown -y`
   - `devx db rm postgres -y`

2. **Parsing Status Output**: Never try to parse `devx vm status` strings using regex. Devx supports structurally deterministic output for AI parsers. You MUST use `--json`:
   - `devx vm status --json`
   - `devx tunnel list --json`
   - `devx db list --json`

3. **Safe Destructive Checks**: When attempting to delete development state (databases, tunnels, VMs), ALWAYS use `--dry-run` first to test exactly what the tool will do, echo it to the user, and ask for confirmation before submitting the destructive `-y` flag.
   - `devx db rm redis --dry-run`
   - `devx vm teardown --dry-run`

4. **Exit Codes**: Devx silences error usage dumps and provides programmatic exit codes. Trap these explicitly in your terminal evaluation loop:
   - `Exit 15`: VM is asleep/dormant.
   - `Exit 16`: VM does not exist (Requires you to run `devx init`).
   - `Exit 22`: Port Address Collision on the host (`devx db spawn <engine>` failed).
   - `Exit 41`: Cloudflared is unauthenticated on the dev machine.
