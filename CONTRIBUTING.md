# Contributing to devx

Thank you for your interest in contributing to devx! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Reporting Bugs](#reporting-bugs)
- [Requesting Features](#requesting-features)
- [Contributing with AI Agents](#contributing-with-ai-agents)

## Code of Conduct

This project follows our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Getting Started

1. **Fork the repository** and clone your fork locally
2. **Create a branch** from `main` for your changes
3. **Make your changes** and write tests where applicable
4. **Submit a pull request** back to the `main` branch

## Development Setup

### Prerequisites

- **Go 1.22+** — [install Go](https://go.dev/dl/)
- **Mage** — `go install github.com/magefile/mage@latest`
- **Podman** — `brew install podman` (macOS) or [install Podman](https://podman.io/docs/installation)
- **cloudflared** — `brew install cloudflare/cloudflare/cloudflared`
- **butane** — `brew install butane`

### Building

```bash
# Clone your fork
git clone https://github.com/<your-username>/devx.git
cd devx

# Build the binary
mage build

# Run tests
mage test

# Run the full CI gate (vet → test → build)
mage ci
```

### Project Structure

```
devx/
├── cmd/                    # CLI command definitions (Cobra)
│   ├── root.go             # Root command & group registration
│   ├── vm.go               # `devx vm` parent command (--provider flag)
│   ├── tunnel.go           # `devx tunnel` parent command
│   ├── config.go           # `devx config` parent command
│   ├── exec.go             # `devx exec` parent command
│   ├── up.go               # `devx tunnel up` (devx.yaml topology)
│   └── ...                 # Individual subcommands
├── internal/               # Private packages
│   ├── authproxy/          # Built-in basic auth reverse proxy
│   ├── cloudflare/         # Cloudflare tunnel management
│   ├── config/             # Runtime configuration
│   ├── exposure/           # Local exposure state store
│   ├── ignition/           # Butane/Ignition config builder
│   ├── inspector/          # HTTP request inspector (TUI)
│   ├── podman/             # Podman machine management (legacy)
│   ├── prereqs/            # Prerequisite validation
│   ├── provider/           # VMProvider interface + backends
│   ├── secrets/            # .env secret management
│   ├── state/              # State replication and checkpoints (Idea 56)
│   ├── tailscale/          # Tailscale agent management
│   └── tui/                # Terminal UI components
├── main.go                 # Entry point
├── magefile.go             # Build automation (Mage)
├── devx.yaml.example       # Example project topology config
├── .goreleaser.yml         # Release configuration
└── dev-machine.template.bu # Fedora CoreOS Butane template
```

## Making Changes

### Code Style

- Follow standard Go conventions ([Effective Go](https://go.dev/doc/effective_go))
- Run `go vet ./...` before committing
- Run `go test ./...` and ensure all tests pass
- Keep functions focused and packages cohesive

### Adding a New Command

1. Create a new file in `cmd/` (e.g., `cmd/mycommand.go`)
2. Register it under the appropriate parent command (`vmCmd`, `tunnelCmd`, `configCmd`, or `execCmd`)
3. Follow the existing command patterns for consistency
4. Add tests if the command contains non-trivial logic

### Adding Internal Packages

- All internal packages go under `internal/` and are not importable by external consumers
- Keep packages small and focused on a single responsibility
- Add `_test.go` files alongside the implementation

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring without behavior changes
- `test`: Adding or updating tests
- `chore`: Build process, CI, or tooling changes

**Examples:**
```
feat(tunnel): add port display to tunnel list output
fix(vm): handle podman machine not found gracefully
docs: update README with new command hierarchy
chore(ci): add golangci-lint to validation workflow
```

## Pull Request Process

1. **Update documentation** if your change affects user-facing behavior
2. **Add tests** for new functionality
3. **Ensure CI passes** — all checks must be green before merge
4. **One approval required** — at least one maintainer must approve
5. **Squash and merge** — PRs are squash-merged to keep history clean

### PR Title

Use the same format as commit messages. The PR title becomes the squash commit message.

### PR Description

Include:
- **What** the change does
- **Why** it's needed
- **How** to test it (if not obvious)

## Reporting Bugs

Open an issue using the **Bug Report** template. Include:

- Your OS and architecture (`go env GOOS GOARCH`)
- devx version (`devx version`)
- Steps to reproduce
- Expected vs. actual behavior
- Relevant logs or error output

## Requesting Features

Open an issue using the **Feature Request** template. Include:

- The problem you're trying to solve
- Your proposed solution (if any)
- Alternative approaches you've considered

## Contributing with AI Agents

AI coding agents (Gemini CLI, Claude Code, Cursor, GitHub Copilot Workspace, etc.) are welcome contributors to `devx`. This section documents the exact workflow an agent should follow when implementing a feature from the project's `IDEAS.md` roadmap.

### Quick-Start Prompt

Copy and paste the following prompt to your AI agent to implement the next feature on the roadmap:

```
Implement the next uncompleted idea from IDEAS.md in the devx CLI repository.
Follow these steps exactly:

## 1. UNDERSTAND THE FEATURE
- Read IDEAS.md and identify the next idea that is NOT marked (DONE).
- Read the existing codebase to understand how similar features are structured.
- Identify which files need to be created or modified.

## 2. CREATE A FEATURE BRANCH
- Branch from main: `git checkout -b feat/<short-description>`

## 3. IMPLEMENT THE FEATURE
- Write the implementation code following existing patterns in the codebase.
- All internal packages go under `internal/`. Commands go under `cmd/`.
- Follow Go conventions and keep packages focused on a single responsibility.

## 4. WRITE TESTS
- Add unit tests for any new packages or non-trivial logic.
- Run the full test suite: `go test -race ./...`
- Fix any failures before proceeding.

## 5. VERIFY THE BUILD
- Compile the binary: `go build -o devx .`
- Run `go mod tidy` if dependencies changed.
- Fix any compilation errors.

## 6. UPDATE DOCUMENTATION (do NOT skip this)
- Mark the idea as (DONE) in IDEAS.md.
- Update README.md with:
  - New commands added to the CLI Reference table.
  - A dedicated feature section with description and usage examples.
  - At least 2-3 practical `bash` code block examples showing real usage.
- Update devx.yaml.example if the feature adds new YAML configuration keys.

## 7. COMMIT AND PUSH
- Stage all changes: `git add -A`
- Commit with conventional commit format: `git commit -m "feat: <description>"`
- Push the branch: `git push -u origin feat/<short-description>`

## 8. CREATE A PULL REQUEST
- Create the PR via CLI: `gh pr create --title "feat: <description>" --body "<summary>"`
- The PR title becomes the squash commit message, so make it descriptive.

## 9. WAIT FOR CI TO PASS
- Watch all pipeline checks: `gh pr checks --watch`
- All 7 checks must pass (Lint, Test, Validate Butane, Build x4 platforms).
- If any check fails, read the logs, fix the issue, amend the commit, and force-push.

## 10. SQUASH MERGE
- Once all checks are green: `gh pr merge --admin --squash --delete-branch`
- Switch back to main: `git checkout main && git pull`

## 11. TAG A NEW RELEASE
- Determine the version bump:
  - Patch (v0.x.Y) for docs-only or small additions.
  - Minor (v0.X.0) for new features or architectural changes.
  - Major (vX.0.0) for breaking changes.
- Tag it: `git tag -a v<version> -m "Release v<version> (<feature name>)"`
- Push the tag: `git push origin v<version>`
- This triggers the GoReleaser pipeline that publishes binaries to GitHub Releases.

## 12. VERIFY THE RELEASE
- Confirm the release pipeline triggered: `gh run list --workflow=release.yml -L 1`
- Optionally watch it: `gh run watch`

## IMPORTANT RULES
- Never push directly to main. Always use feature branches and PRs.
- Always wait for CI to pass before merging. Never skip checks.
- Always update README.md with examples. A feature without docs is incomplete.
- Always mark the idea as (DONE) in IDEAS.md after merging.
- Keep commits atomic. One feature = one squash commit on main.
- Use `go mod tidy` after adding any new dependencies.
- Run `go build -o devx .` locally before pushing to catch errors early.
```

### Pipeline Checks Reference

The CI pipeline runs the following checks on every PR. All must pass:

| Check | What It Validates |
|-------|-------------------|
| `CI/Lint` | `golangci-lint` — code quality and style |
| `CI/Test` | `go test -race ./...` — unit tests with race detection |
| `CI/Validate Butane Template` | Butane YAML template compilation |
| `CI/Build (darwin, amd64)` | macOS Intel binary compilation |
| `CI/Build (darwin, arm64)` | macOS Apple Silicon binary compilation |
| `CI/Build (linux, amd64)` | Linux x86_64 binary compilation |
| `CI/Build (linux, arm64)` | Linux ARM64 binary compilation |

### Release Pipeline

Tagging a version (e.g., `git tag -a v0.3.0`) triggers the `release.yml` workflow which uses [GoReleaser](https://goreleaser.com/) to:

1. Cross-compile binaries for all supported platforms
2. Create a GitHub Release with auto-generated changelog
3. Upload `.tar.gz` archives as release assets

## License

By contributing to devx, you agree that your contributions will be licensed under the [MIT License](LICENSE).
