# AGENTS.md

Guidance for AI coding agents working in **devx** (useful for humans too). devx is a standalone
Go CLI. **This repo is plain Go + [Mage](https://magefile.org) — there are no Bazel `BUILD`
files here.** (Bazel/gazelle only apply when devx is synced into the vitruvian-core monorepo via
Copybara; a `//:tidy`-style commit may ride in from the monorepo side — ignore it here.)

## Build, test, run
devx is itself a developer-experience tool — **dogfood it for the inner loop, then fall back to
Mage (the documented task runner, which wraps `go`) → raw `go` if devx is unavailable or fails**
(e.g. no container runtime in a sandbox).

- **Validate like CI before pushing:** `devx ci run` — runs the repo's GitHub Actions
  (`.github/workflows/ci.yml`: lint + test + butane) locally, with matrix/DAG expansion.
  Fallback: `mage ci`; last resort, run the jobs directly (below).
- **Environment health:** `devx doctor` (audits/installs prerequisites). devx also installs
  `pre-commit` / `pre-push` git hooks that run these checks automatically.
- **Build:** `mage build` (→ `go build -ldflags="-s -w …" -o devx .`); fallback `go build -o devx .`.
- **Unit tests:** `mage test` (→ `go test -race -coverprofile=coverage.out ./...`); fallback
  `go test -race ./...`. (Note: `devx test` runs *isolated test topologies against ephemeral
  environments* — not these Go unit tests.)
- **Lint:** `golangci-lint run` (what CI enforces); `mage lint` is `go vet` only — not a full substitute.
- **Run / iterate:** `go run . <cmd>` or `mage run`; `devx run -- <cmd>` wraps a command with
  telemetry + secret injection if you want that.
- **Tidy deps:** `mage tidy` or `go mod tidy`.

## Conventions & landmines
- **Do not add Bazel `BUILD` / `MODULE.bazel` files** — BUILD generation happens on the monorepo
  side; adding them here will fight the Copybara sync.
- **Standalone-only files must never leak into the monorepo:** the dispatch workflow
  `.github/workflows/sync-to-monorepo.yaml` (and any `package-lock.json`) are standalone-only by design.
- **Conventional commits** — releases and `CHANGELOG.md` are automated via **release-please**; that
  changelog is how monorepo maintainers track what to port.
- Keep dependencies lean and `go.mod` tidy.

## Docs
- Design/plan docs live in `docs/` (`docs/plans/`, `docs/guide/`). Plans marked `status: completed`
  are done — don't treat them as open TODOs.
