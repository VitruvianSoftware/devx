# Security Auditing (`devx audit`)

`devx audit` scans your project for two of the most common pre-push mistakes — leaked credentials and vulnerable dependencies — before they reach CI or, worse, production.

Both scan tools run **natively** if installed, or fall back to an **ephemeral container** automatically. No installation required.

## Quick Start

```bash
devx audit
```

```
devx audit

  ▸ Gitleaks  —  native
  ...
  ✓ PASS  (340ms)

  ▸ Trivy  —  container (podman)
  2024-01-15T10:14:03Z INFO Vulnerability scanning is enabled
  ...
  ✗ FAIL  (4.2s — issues found)

✗ 1 scan(s) found issues — review output above before pushing.
```

## Commands

| Command | Description |
|---|---|
| `devx audit` | Run both scans (secrets + vulns) |
| `devx audit secrets` | Secrets scan only (Gitleaks) |
| `devx audit vulns` | Vulnerability scan only (Trivy) |
| `devx audit install-hooks` | Wire into git pre-push hook |

## Secret Scanning (Gitleaks)

`devx audit secrets` runs [Gitleaks](https://github.com/gitleaks/gitleaks) against your working tree, catching hardcoded credentials before they reach git history:

```bash
devx audit secrets
```

What it catches:
- AWS/GCP/Azure access keys and secrets
- API tokens (GitHub, Stripe, Twilio, SendGrid, etc.)
- Private keys and certificates
- Database connection strings with passwords
- JWT secrets

::: tip What about .env files?
Gitleaks will flag `.env` files if they contain real secrets. If your project has a committed `.env.example` with placeholder values (e.g. `STRIPE_KEY=sk_test_REPLACE_ME`), add a `.gitleaks.toml` to exclude it:

```toml
# .gitleaks.toml
[allowlist]
  paths = [".env.example"]
```
:::

## Vulnerability Scanning (Trivy)

`devx audit vulns` runs [Trivy](https://trivy.dev) in filesystem mode across your entire project, scanning all language dependencies:

```bash
devx audit vulns
```

Languages supported:
| Language | Scanned Files |
|---|---|
| **Go** | `go.sum` |
| **Node.js** | `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml` |
| **Python** | `requirements.txt`, `Pipfile.lock`, `poetry.lock` |
| **Rust** | `Cargo.lock` |
| **Java** | `pom.xml`, `build.gradle` |

## Native vs Container Mode

`devx audit` automatically selects the fastest available execution method:

```
Is trivy/gitleaks in $PATH?
  YES → Run natively (fastest, ~100ms overhead)
  NO  → Pull image once, run in ephemeral read-only container
          └── prefers podman, falls back to docker
```

The container mount is **read-only** (`-v $(pwd):/scan:ro`) and network-isolated (`--network none`) — the scanner cannot write to your project or make outbound calls.

To force a specific runtime:
```bash
devx audit --runtime docker
```

## Git Pre-Push Hook

Run once to wire `devx audit` into your git workflow:

```bash
devx audit install-hooks
```

This writes `.git/hooks/pre-push`:
```sh
#!/bin/sh
set -e
echo "🔍 devx audit: scanning for secrets and vulnerabilities..."
devx audit
```

Every subsequent `git push` will run the audit. If issues are found, the push is **aborted** (exit code 1). Fix the findings and push again.

::: tip Share across the team
Add a one-liner to your project's `CONTRIBUTING.md` or `Makefile`:
```bash
make setup:  ## Run once after cloning
	devx audit install-hooks
```
:::

## Flags

| Flag | Description |
|---|---|
| `--secrets` | Run secrets scan only |
| `--vulns` | Run vulnerability scan only |
| `--runtime` | Force container runtime (`podman` or `docker`) |
| `--json` | Output results as JSON (for CI parsers) |

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | All scans passed — safe to push |
| `1` | One or more scans found issues |

This makes `devx audit` composable in Makefiles and CI:
```bash
# Makefile
pre-push:
	devx audit || (echo "Fix audit failures before pushing"; exit 1)
```

## Why Not Just Rely on GitHub CI?

| | `devx audit` (local) | GitHub Actions (CI) |
|---|---|---|
| Feedback speed | **~5 seconds** | 2–10 minutes |
| Fixes context-switch | ✓ (still in editor) | ✗ (interrupts next task) |
| Blocks push | ✓ (pre-push hook) | ✗ (after push) |
| Works offline | ✓ (native mode) | ✗ |
| Catches secrets pre-history | **✓ (before commit)** | ✗ (already in git) |

The goal isn't to replace CI scanning — it's to catch the 90% of issues that are simple to fix in 30 seconds locally but become expensive PR review cycles when they reach CI.
