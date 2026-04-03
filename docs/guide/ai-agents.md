# AI Agent Skills

`devx` is designed to be **AI-native** — it includes built-in support for configuring AI coding agents with project-specific knowledge and standard operating procedures through an extensible agent skills system.

## What Are Agent Skills?

Agent skills are structured markdown files that teach AI coding assistants about your project's conventions, workflows, and SOPs. They live in your repository and are automatically discovered by tools like [Antigravity/Gemini CLI](https://github.com/google-gemini/gemini-cli), [Claude Code](https://docs.anthropic.com/en/docs/claude-code), Cursor, and GitHub Copilot.

Each skill targets a specific concern — keeping CLI tooling rules separate from general engineering best practices.

## Quick Start

Run the interactive installer:

```bash
devx agent init
```

This launches a **two-step TUI**:

**Step 1** — Pick which AI agents you use:

```
Which AI Agent(s) do you use?
  [•] Antigravity/Gemini (Standard Agent Skills)
  [ ] Cursor IDE
  [ ] Claude Code (Anthropic)
  [ ] GitHub Copilot Chat
```

**Step 2** — Pick which skills to inject:

```
Which skills should we inject?
  [•] Devx CLI Orchestrator Rules — Mandates --json, --dry-run, and handles prediction of devx exit codes.
  [•] Platform Engineering SOP (Mandatory Docs) — Enforces strict documentation-first behavior and image embedding requirements.
```

`devx` then writes the appropriate `SKILL.md` files into each agent's config directory:

| Agent | Skill destination |
|---|---|
| Antigravity/Gemini | `.agent/skills/<skill>/SKILL.md` |
| Cursor | `.cursor/skills/<skill>/SKILL.md` |
| Claude Code | `.claude/skills/<skill>/SKILL.md` |
| GitHub Copilot | `.github/skills/<skill>/SKILL.md` |

## Force Reinstall

If a skill file already exists, `devx agent init` will skip it safely. To overwrite:

```bash
devx agent init --force
```

## Available Skills

### `devx` — Devx CLI Orchestrator Rules

Teaches AI agents how to interact with the `devx` CLI correctly:

- Always use `--json` for machine-readable output
- Always use `--non-interactive` / `-y` to avoid TTY stalls
- Use `--dry-run` before destructive operations
- How to interpret devx numeric exit codes (e.g. `Exit 22: Port in Use`)

### `platform-engineer` — Platform Engineering SOP

Enforces team-wide platform engineering best practices:

- **Mandatory Documentation Policy** — Agents must proactively update official docs (`docs/`, `FEATURES.md`) after any successful verification or feature implementation. Never ask; just do it.
- **Visual Proof** — Screenshots and terminal output from verifications must be embedded in documentation.
- **Completion Criteria** — A task is only DONE after docs reflect the new state.

## Adding New Skills

New skills are embedded directly into the `devx` binary at compile time. To add a skill:

1. Create `internal/agent/templates/.<agent>/skills/<skill-name>/SKILL.md` for each agent platform.
2. Add an entry to `AvailableSkills` in `internal/agent/embed.go`.

The next `devx agent init` will offer the new skill automatically.

## Why It Matters

When an AI agent opens your project, it immediately reads these skill files to understand your architecture and rules — without needing to read the entire codebase first. It also enforces team standards that would otherwise need to be repeated in every prompt.

## `devx agent ship` — Deterministic Pipeline Guardrail

AI agents have a fundamental weakness: they forget to verify CI pipelines after merging code. `devx agent ship` eliminates this by wrapping the entire commit → push → PR → CI lifecycle into a single blocking command.

### Usage

```bash
devx agent ship -m "feat: implement new feature"
```

This command executes four phases sequentially:

| Phase | Description |
|---|---|
| **Pre-flight** | Runs local tests, lint, and build for the auto-detected stack |
| **Commit & Push** | Stages, commits, and pushes (bypassing the pre-push hook internally) |
| **PR & Merge** | Creates a GitHub PR and squash-merges it |
| **CI Poll** | **Blocks the terminal** until the CI pipeline completes on main |

The command returns deterministic exit codes:

| Exit Code | Meaning |
|---|---|
| `0` | Success — CI is green |
| `50` | Pre-flight failure (tests/lint/build) |
| `51` | Git push failed |
| `52` | PR creation or merge failed |
| `53` | CI pipeline failed |
| `54` | CI pipeline timed out |
| `55` | Documentation check failed |
| `56` | Nothing to ship |

### Machine-Readable Output

```bash
devx agent ship -m "fix: resolve bug" --json
```

### Pre-Push Hook (The Forcing Function)

To prevent agents (or forgetful humans) from bypassing `devx agent ship`:

```bash
devx agent ship --install-hook
```

This installs a `.git/hooks/pre-push` hook that **blocks all direct `git push` commands**. When triggered, it prints:

```
✋ Direct 'git push' is blocked by devx.
   AI Agents MUST use:   devx agent ship -m "commit message"
   Humans can bypass:    git push --no-verify
```

The hook is automatically detected by `devx doctor`, which will warn if it's missing.
