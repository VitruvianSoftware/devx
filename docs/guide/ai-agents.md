# AI Agent Skills

`devx` is designed to be **AI-native** — it includes built-in support for configuring AI coding agents with project-specific knowledge through the agent skills system.

## What Are Agent Skills?

Agent skills are structured markdown files (`.md`) that teach AI coding assistants about your project's architecture, conventions, and workflows. They live in your repository and are automatically discovered by tools like [Gemini CLI](https://github.com/google-gemini/gemini-cli), [Claude Code](https://docs.anthropic.com/en/docs/claude-code), and similar agentic coding tools.

## Commands

### `devx agent`

Manage AI agent configuration for your project:

```bash
devx agent
```

## Skill File Format

Each skill file uses YAML frontmatter followed by markdown instructions:

```markdown
---
name: Backend API Development
description: Conventions for building REST APIs in this project
---

## API Route Patterns

All routes follow the pattern `/api/v1/{resource}`.

## Error Handling

Use the `AppError` struct from `internal/errors` for all error responses.
```

## Directory Structure

Skills are stored in your project's `.agent/skills/` or `.agents/skills/` directory:

```
.agent/
└── skills/
    ├── backend-api.md
    ├── database-migrations.md
    ├── testing-conventions.md
    └── deployment-workflow.md
```

## Why It Matters

When an AI agent opens your project, it reads these skill files to understand:

- **Architecture decisions** — Why things are structured the way they are
- **Code conventions** — Naming patterns, error handling, logging standards
- **Workflows** — How to test, deploy, and review changes
- **Gotchas** — Known issues, workarounds, and anti-patterns to avoid

This dramatically reduces the time an AI spends "learning" your codebase and produces more accurate, project-consistent code.

## Integration with devx

The `devx` project itself uses agent skills to document its own patterns:

- CLI command structure and Cobra conventions
- Ignition/Butane template patterns
- Cloudflare and GitHub API client patterns
- Testing and linting requirements

When contributors (human or AI) work on `devx`, these skills provide immediate context without reading the entire codebase.
