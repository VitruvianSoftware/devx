# MCP Server for AI Agents

`devx mcp` exposes the entire devx command surface as a [Model Context Protocol](https://modelcontextprotocol.io) server, so AI coding agents (Claude Code, Cursor, Codex CLI, Gemini CLI, Zed, Continue.dev, Windsurf, Cowork, Antigravity, OpenCode, and any other MCP-aware host) can call devx commands as **first-class typed tools** instead of shelling out and parsing text.

## Why

Without MCP, every agent that wants to use devx has to learn the CLI: always append `--json`, always pass `-y` because there's no TTY, parse exit codes, handle prompt-confirm-prompt loops. That's brittle and platform-specific.

With MCP, the agent's host translates calls into typed JSON-RPC requests. Tool schemas tell the agent exactly what arguments to pass; structured return values replace text parsing; destructive operations are marked so the host can prompt the user before they run. The agent stops being a "careful CLI consumer" and becomes a direct caller of devx primitives.

## Install everywhere in one command

```bash
devx mcp install
```

Auto-detects every installed agent host on your machine and configures each one. Idempotent — re-running is safe. Hosts that support per-project config (Claude Code, Cursor, OpenCode) are configured at the project level by default so each repo's devx tool surface is sandboxed and travels with the codebase.

Example output:

```
📝 devx MCP install
   ✓ claude-code     → ./.mcp.json [project]  (added devx MCP entry)
   ✓ cursor          → ./.cursor/mcp.json [project]  (added devx MCP entry)
   ✓ codex           → ~/.codex/config.toml [user]  (added devx MCP entry)
   • gemini-cli      → ~/.gemini/settings.json [user]  (already up to date)

  Restart your agent (or open a new chat) to load devx tools.
```

## Targeted install

```bash
devx mcp install claude-code         # install for just one host
devx mcp install claude-code cursor  # install for several
devx mcp install --all               # install for every supported host, even undetected
devx mcp install --global            # use per-user config (overrides project-scoped default)
devx mcp install --dry-run           # show what would change without writing
devx mcp install --json              # machine-readable output
```

## Supported hosts

| Key | Tool | Config Path | Format | Scope |
|-----|------|-------------|--------|-------|
| `claude-code` | Claude Code | `.mcp.json` or `~/.claude.json` | JSON | project / user |
| `cursor` | Cursor | `.cursor/mcp.json` or `~/.cursor/mcp.json` | JSON | project / user |
| `codex` | OpenAI Codex CLI | `~/.codex/config.toml` | TOML | user |
| `gemini-cli` | Google Gemini CLI | `~/.gemini/settings.json` | JSON | user |
| `zed` | Zed editor | `~/.config/zed/settings.json` (key `context_servers`) | JSON | user |
| `continue` | Continue.dev | `~/.continue/config.json` | JSON | user |
| `windsurf` | Windsurf (Codeium) | `~/.codeium/windsurf/mcp_config.json` | JSON | user |
| `opencode` | OpenCode | `.opencode/mcp.json` or `~/.config/opencode/mcp.json` | JSON | project / user |
| `cowork` | Cowork | plugin/connector UI | manual snippet | manual |
| `antigravity` | Google Antigravity IDE | settings UI | manual snippet | manual |

Hosts marked "manual" use plugin/marketplace integration models rather than flat config files. For those, `devx mcp install` prints a copy-pasteable JSON snippet plus host-specific instructions.

## Status and uninstall

```bash
devx mcp status              # show install state for every supported host
devx mcp uninstall           # remove from every detected host
devx mcp uninstall codex     # remove from one host
devx mcp doctor              # spawn the server, send a tools/list, confirm it works
devx mcp list                # list supported hosts + registered tools
```

`devx mcp doctor` actually starts `devx mcp serve` in a subprocess, performs the MCP handshake over stdio, and confirms the expected tools come back. Run it right after `devx mcp install` to catch problems before opening your agent.

## What the agent sees

After install, your agent host shows ~18 `devx_*` tools, organized by capability:

**Inspection** — `devx_doctor`, `devx_vm_status`, `devx_db_list`, `devx_cloud_list`, `devx_tunnel_list`, `devx_config_validate`, `devx_config_pull`. All marked `readOnlyHint: true` so the host knows they're safe to call without confirmation.

**Provisioning** — `devx_db_spawn`, `devx_cloud_spawn`, `devx_db_snapshot_create`, `devx_state_share`, `devx_ci_run`. Safe and idempotent.

**Destructive** (host should prompt before calling) — `devx_db_snapshot_restore`, `devx_state_attach`, `devx_ship`, `devx_db_rm`, `devx_cloud_rm`, `devx_vm_teardown`. All marked `destructiveHint: true`.

Each tool has a typed input schema (engine names, port numbers, etc.) so the agent never has to guess the right flag.

## How it works

`devx mcp serve` is a long-running stdio process. The agent host spawns it as a subprocess and exchanges JSON-RPC 2.0 messages on stdin/stdout. Each tool call shells out to the devx binary itself (the same one being served from) with the requested arguments, captures stdout, and returns it as the tool result. Subprocess isolation keeps the MCP server's own stdout (the JSON-RPC transport) free of any cobra-generated chatter.

```
┌──────────────┐   stdio JSON-RPC   ┌──────────────┐   subprocess   ┌──────────┐
│  Agent host  │ ←──────────────→  │ devx mcp     │ ─────────────→ │ devx ... │
│ (Claude/etc) │                    │   serve      │ ←─ stdout  ──── │ --json   │
└──────────────┘                    └──────────────┘                 └──────────┘
```

## Manual install (advanced)

If you want to wire devx into a host devx doesn't yet support, the config snippet is always:

```json
{
  "mcpServers": {
    "devx": {
      "command": "/absolute/path/to/devx",
      "args": ["mcp", "serve"]
    }
  }
}
```

Get the absolute path with `which devx`. The container key (`mcpServers`) varies by host — Zed uses `context_servers`, Codex uses `[mcp_servers.devx]` in TOML, etc. See `devx mcp install --json` output (or the table above) for the exact format your host expects.
