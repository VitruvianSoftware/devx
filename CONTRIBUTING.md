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
│   ├── vm.go               # `devx vm` parent command
│   ├── tunnel.go           # `devx tunnel` parent command
│   ├── config.go           # `devx config` parent command
│   ├── exec.go             # `devx exec` parent command
│   └── ...                 # Individual subcommands
├── internal/               # Private packages
│   ├── cloudflare/         # Cloudflare tunnel management
│   ├── config/             # Runtime configuration
│   ├── exposure/           # Local exposure state store
│   ├── ignition/           # Butane/Ignition config builder
│   ├── podman/             # Podman machine management
│   ├── prereqs/            # Prerequisite validation
│   ├── secrets/            # .env secret management
│   ├── tailscale/          # Tailscale agent management
│   └── tui/                # Terminal UI components
├── main.go                 # Entry point
├── magefile.go             # Build automation (Mage)
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

## License

By contributing to devx, you agree that your contributions will be licensed under the [MIT License](LICENSE).
