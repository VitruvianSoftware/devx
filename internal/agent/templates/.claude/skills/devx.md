---
description: "Agent Skill: Orchestrate Local Environment (VM, DBs, Cloudflare Tunnels)"
argument-hint: Ask the user to clarify what devx component they are seeking to manage if unclear.
---
# Claude Code AI Guidelines for Devx Local Dev Stack

You are orchestrating a `devx` managed local environment built on Podman, Cloudflare Tunnels, and Tailscale.

When interacting with development databases or environment networks, ALWAYS use the `devx` CLI interface rather than raw podman/docker shell commands.

## Mandatory Automation Flags
1. **Never stall on TUI Prompts**: You do not have a TTY interface. Many commands (like `teardown` or missing secrets files) use terminal UI components that will block you forever. You MUST append `--non-interactive` (or `-y`) to mutating commands:
   - `devx init -y`
   - `devx vm teardown -y`
   - `devx db rm postgres -y`

2. **JSON Machine-Readable Parsing**: Never parse human-readable TUI output strings using regex. Devx supports structurally deterministic tracking for AI parsers. You MUST use `--json`:
   - `devx vm status --json`
   - `devx tunnel list --json`
   - `devx db list --json`

3. **Safe Destructive Validations**: When attempting to delete development state (databases, tunnels, VMs), ALWAYS use `--dry-run` to test exactly what the tool will do, echo it to the user, and ask for confirmation before submitting the destructive `-y` flag.
   - `devx db rm redis --dry-run`
   - `devx tunnel unexpose --dry-run`

4. **Exit Codes**: Devx silences error usage dumps and provides programmatic custom exit codes. Ensure you trap these in explicit `if` statements:
   - `Exit Status 15 (CodeVMDormant)`: VM exists but is sleeping.
   - `Exit Status 16 (CodeVMNotFound)`: VM has been deleted. (Run `devx init`).
   - `Exit Status 22 (CodeHostPortInUse)`: Host port address collision (`devx db spawn <engine>` failed).
   - `Exit Status 41 (CodeNotLoggedIn)`: Cloudflared is unauthenticated on the dev machine.
