# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.20.0](https://github.com/VitruvianSoftware/devx/compare/v0.19.0...v0.20.0) (2026-04-01)


### Features

* **ci:** automate versioning and changelog generation via release-please ([#68](https://github.com/VitruvianSoftware/devx/issues/68)) ([d717ac2](https://github.com/VitruvianSoftware/devx/commit/d717ac290100fdec6f4a06195a298dba3a5382f5))

## [0.19.0] - 2026-04-01

### Added
- **Instant Security Auditing** (`devx audit`)
  - Pre-push vulnerability (Trivy) and secret (Gitleaks) scanning
  - Zero-install architecture runs missing tools automatically via ephemeral read-only Podman/Docker containers
  - One-line git hooks integration (`devx audit install-hooks`)
  - Bypasses `gcloud` credential helper conflicts securely for public images
- **Zero-Friction Production Data Sync** (`devx db pull`)
  - Pulls pre-anonymized databases directly into local containers
  - New parallel binary ingestion mode (`pg_restore -j <N>`) for 5GB+ Postgres databases
  - Standard SQL streaming for MySQL/MongoDB operations
- **AI Agent Tooling & Workflows** (`v0.8.0` - `v0.15.0`)
  - Official agent skills directory (`.agents/skills`) with `--force` upgrade system
  - Predictable exit codes and unified JSON output hooks (`--json`)
  - Global AI override flags (`--dry-run`, `--non-interactive`)
- **Documentation Site**
  - Deployed comprehensive Vitepress documentation site matching the CLI feature set
- **Site Deployment** (`devx sites init`)
  - Automated GitHub Pages and Cloudflare DNS provisioning via interactive wizard
- **Advanced Infrastructure**
  - Devcontainer integration (`devx shell`)
  - Multi-port topology parsing via `devx.yaml`
  - Built-in basic auth for exposed tunnels
  - One-click database provisioning (`devx db spawn`)
  - Vault-backed secret synchronization and `.env` automation
  - Native network simulation for fault injection

### Fixed
- Lint errors (`ineffassign`) and dead links in Vitepress build CI pipelines
- Resolved edge cases connecting to sleeping Podman machines during container executions
- Accidental interception of public container registry pulls by gcloud auth helpers

## [0.2.0] - 2026-03-30

### Added
- **Request Inspector TUI** (`devx tunnel inspect [port]`) — a free, open-source replacement for ngrok's paid web inspector
  - Live reverse proxy captures all HTTP request/response pairs
  - Beautiful terminal UI with scrollable request list and detail view
  - One-key replay to resend captured requests
  - Replay tagging to distinguish original vs replayed traffic
  - Optional Cloudflare tunnel exposure via `--expose` flag
  - Full header and body inspection with syntax-aware display
- CHANGELOG.md
- IDEAS.md roadmap document

### Removed
- IMPROVEMENTS.md (replaced by IDEAS.md)

## [0.1.0] - 2026-03-30

### Added
- Initial open-source release
- Nested CLI hierarchy: `vm`, `tunnel`, `config`, `exec`
- Interactive TUI provisioning with Bubble Tea (`devx vm init`)
- ngrok-like port exposure via Cloudflare tunnels (`devx tunnel expose`)
- Port display in tunnel list output
- Local exposure state store (`~/.config/devx/exposures.json`)
- `devx version` command with build-time version injection
- CI pipeline: golangci-lint, tests, cross-platform build matrix, Butane validation
- Release pipeline: GoReleaser with GitHub releases
- Open-source docs: LICENSE (MIT), CONTRIBUTING, CODE_OF_CONDUCT, SECURITY
- GitHub issue and PR templates
- Branch protection on `main` with required status checks

[0.2.0]: https://github.com/VitruvianSoftware/devx/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/VitruvianSoftware/devx/releases/tag/v0.1.0
