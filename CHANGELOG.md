# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
