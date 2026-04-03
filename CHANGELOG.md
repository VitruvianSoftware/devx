# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.27.3](https://github.com/VitruvianSoftware/devx/compare/v0.27.2...v0.27.3) (2026-04-03)


### Bug Fixes

* **ci:** resolve golangci-lint warnings in ci parser and executor ([#113](https://github.com/VitruvianSoftware/devx/issues/113)) ([88e830c](https://github.com/VitruvianSoftware/devx/commit/88e830ce6b05b2d7e6618daf90d9e1b40d26a8d4))

## [0.27.2](https://github.com/VitruvianSoftware/devx/compare/v0.27.1...v0.27.2) (2026-04-03)


### Bug Fixes

* **docs:** escape vue interpolations in markdown ([605dd89](https://github.com/VitruvianSoftware/devx/commit/605dd892b3715a2b61b4caa43b842fa63155f031))
* **docs:** escape vue interpolations in markdown ([766ceb0](https://github.com/VitruvianSoftware/devx/commit/766ceb08b9395613f915e0424a9c2643cfcf0b00))

## [0.27.1](https://github.com/VitruvianSoftware/devx/compare/v0.27.0...v0.27.1) (2026-04-03)


### Bug Fixes

* **devxerr:** add interactive gcloud auth auto-recovery for container tasks ([#109](https://github.com/VitruvianSoftware/devx/issues/109)) ([e8df755](https://github.com/VitruvianSoftware/devx/commit/e8df7554d474addc761ce36328aab0a5bea50e71))

## [0.27.0](https://github.com/VitruvianSoftware/devx/compare/v0.26.0...v0.27.0) (2026-04-03)


### Features

* implement devx ci run — local CI pipeline emulation (Idea 42) ([#107](https://github.com/VitruvianSoftware/devx/issues/107)) ([076e737](https://github.com/VitruvianSoftware/devx/commit/076e737ebb8d14ec59b526918df8d683025cbaf1))

## [0.26.0](https://github.com/VitruvianSoftware/devx/compare/v0.25.0...v0.26.0) (2026-04-03)


### Features

* Devx State Command Hierarchy (Idea 41 & 47) ([da85676](https://github.com/VitruvianSoftware/devx/commit/da85676eb182c2ab202f2ea87f16861c6d07dc7e))
* implement devx agent ship — deterministic agentic pipeline guardrail (Idea 40) ([#105](https://github.com/VitruvianSoftware/devx/issues/105)) ([f9251ba](https://github.com/VitruvianSoftware/devx/commit/f9251badb8b6aae4261c182d5992f888cff82e52))


### Bug Fixes

* apply strict documentation checks to embed template ([#100](https://github.com/VitruvianSoftware/devx/issues/100)) ([3afe719](https://github.com/VitruvianSoftware/devx/commit/3afe719a9db593427dc468fad48eda68cc43b50e))
* downgrade go.mod to 1.24.0 to resolve golangci-lint CI failure ([#104](https://github.com/VitruvianSoftware/devx/issues/104)) ([b54bd26](https://github.com/VitruvianSoftware/devx/commit/b54bd26887e1eeac0ec9edf627c771b925f05689))
* resolve 3 bugs and 4 design issues in devx state implementation ([5fda58e](https://github.com/VitruvianSoftware/devx/commit/5fda58e7ed9f2142753e281c9ae55b3937ecfa0f))

## [0.25.0](https://github.com/VitruvianSoftware/devx/compare/v0.24.0...v0.25.0) (2026-04-03)


### Features

* implement DAG-based service orchestration (Ideas 34/35/36) ([#94](https://github.com/VitruvianSoftware/devx/issues/94)) ([e86d663](https://github.com/VitruvianSoftware/devx/commit/e86d663e486dddf3f011f582097e1a211832439a))
* P1 Polish Pass — Environment Profiles, Secrets Redaction, Visual Map (Ideas 37/38/39) ([#97](https://github.com/VitruvianSoftware/devx/issues/97)) ([4722b7f](https://github.com/VitruvianSoftware/devx/commit/4722b7fd567fd8ee936cb81c148db152f5f259c7))


### Bug Fixes

* handle errcheck lint failures in DAG test ([#96](https://github.com/VitruvianSoftware/devx/issues/96)) ([5f82d00](https://github.com/VitruvianSoftware/devx/commit/5f82d00956397c546ed15b7118d49929605277a4))

## [0.24.0](https://github.com/VitruvianSoftware/devx/compare/v0.23.0...v0.24.0) (2026-04-02)


### Features

* proactive CLI error resolutions ([3b35ab4](https://github.com/VitruvianSoftware/devx/commit/3b35ab4987cf8eebe0d770b46dea12724b0aabaf))
* proactive user-friendly auto-resolution for common CLI errors ([8498c11](https://github.com/VitruvianSoftware/devx/commit/8498c112483ef497580572558af8683b169a2aa0))

## [0.23.0](https://github.com/VitruvianSoftware/devx/compare/v0.22.0...v0.23.0) (2026-04-02)


### Features

* **k8s:** implement zero-config devx k8s local clusters via single-binary k3s ([#91](https://github.com/VitruvianSoftware/devx/issues/91)) ([c68fa35](https://github.com/VitruvianSoftware/devx/commit/c68fa353e19322a1debf67ccd9cf5b47791b8ea5))
* **mock:** implement devx mock for OpenAPI 3rd-party API mocking ([#89](https://github.com/VitruvianSoftware/devx/issues/89)) ([858ce06](https://github.com/VitruvianSoftware/devx/commit/858ce06337da96f7bbb70d3f99354d514f018e7d))
* **testing:** implement devx test ui for ephemeral browser testing isolation ([#85](https://github.com/VitruvianSoftware/devx/issues/85)) ([0b0a88b](https://github.com/VitruvianSoftware/devx/commit/0b0a88b28e747822d99007372da96a5ca73b7e0c))


### Bug Fixes

* **docs:** copy visual proof image to public asset directory to resolve VitePress CI build error ([#86](https://github.com/VitruvianSoftware/devx/issues/86)) ([8bdc59a](https://github.com/VitruvianSoftware/devx/commit/8bdc59a91fe665090884c5f9fa4192406dd99420))
* **lint:** Ignore error returns in test helpers ([19d4a6f](https://github.com/VitruvianSoftware/devx/commit/19d4a6f7282211ebdd424dd1e16992b8594a0de0))
* **mock:** handle Sscanf error return to satisfy errcheck linter ([#90](https://github.com/VitruvianSoftware/devx/issues/90)) ([23dcda9](https://github.com/VitruvianSoftware/devx/commit/23dcda9251821636c185295a854053f74a6d2162))
* **vault:** convert Bitwarden sync to native Go with auto-login and schema provisioning ([#83](https://github.com/VitruvianSoftware/devx/issues/83)) ([27efafe](https://github.com/VitruvianSoftware/devx/commit/27efafef0f6b791229923c418b3551ea4bbf3286))

## [0.22.0](https://github.com/VitruvianSoftware/devx/compare/v0.21.0...v0.22.0) (2026-04-02)


### Features

* **agent:** multi-skill orchestrator + shift-left observability docs ([903f54e](https://github.com/VitruvianSoftware/devx/commit/903f54e89373f8d4ca050d313392cafed853d71b))
* **trace:** shift-left distributed observability via devx trace ([#81](https://github.com/VitruvianSoftware/devx/issues/81)) ([fcad429](https://github.com/VitruvianSoftware/devx/commit/fcad42955f4555239868d8884b6c0d8eb0640abc))

## [0.21.0](https://github.com/VitruvianSoftware/devx/compare/v0.20.1...v0.21.0) (2026-04-01)


### Features

* **ai:** zero-friction local AI bridge and agentic workflow mounts ([#80](https://github.com/VitruvianSoftware/devx/issues/80)) ([c8f1812](https://github.com/VitruvianSoftware/devx/commit/c8f1812c8753fad951ef5c24a985285d1040c63d))
* **scaffold:** new devx scaffold command with 6 built-in templates ([#74](https://github.com/VitruvianSoftware/devx/issues/74)) ([ca2163f](https://github.com/VitruvianSoftware/devx/commit/ca2163f45e265cc22f0e8b70f0193084e1e0712d))


### Bug Fixes

* **scaffold:** make scaffold idempotent by default ([#76](https://github.com/VitruvianSoftware/devx/issues/76)) ([8ef7098](https://github.com/VitruvianSoftware/devx/commit/8ef70985c9c5f90539bdca4e8e2cf7990958106c))
* **scaffold:** resolve go vet warning for redundant newlines in Println ([#78](https://github.com/VitruvianSoftware/devx/issues/78)) ([a50646e](https://github.com/VitruvianSoftware/devx/commit/a50646e3aebf54d0efa977240a1073c025966042))

## [0.20.1](https://github.com/VitruvianSoftware/devx/compare/v0.20.0...v0.20.1) (2026-04-01)


### Bug Fixes

* **ci:** actually merge goreleaser into release-please ([#73](https://github.com/VitruvianSoftware/devx/issues/73)) ([3dd18e8](https://github.com/VitruvianSoftware/devx/commit/3dd18e80f27476332cbdc846e1252955559c182c))
* **ci:** wire goreleaser directly into release-please pipeline ([#71](https://github.com/VitruvianSoftware/devx/issues/71)) ([29006dd](https://github.com/VitruvianSoftware/devx/commit/29006dd2514f8e53cd9ff1b4fc31d96ff8555650))

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
