# devx: Architecture & Implementation Improvements

This document captures a structured analysis of the current implementation across packaging, testability, validation, and distribution concerns. Issues are grouped by severity with concrete recommended fixes.

---

## Current State

| File | Role |
|---|---|
| `setup-env.sh` | Single monolithic bash script — does everything |
| `dev-machine.template.bu` | Butane config template (vars not interpolated) |
| `dev-machine.bu` | Populated Butane file — **build artifact, should not be committed** |
| `config.ign` | Compiled Ignition JSON — **build artifact, should not be committed** |

---

## 🚨 Critical Issues

### 1. Secrets Baked Into Build Artifacts

`dev-machine.bu` and `config.ign` both contain a **live Cloudflare tunnel token** in plaintext. Both files are **build artifacts** — generated outputs that should never be committed. The template (`dev-machine.template.bu`) is the source of truth; it contains only placeholders and is safe to commit.

There are two separate concerns that need different solutions:
- **Build artifacts** (`dev-machine.bu`, `config.ign`): gitignore them — they are outputs, not source
- **Secret values** (the actual token): need a proper local secret management strategy

#### Option A: Plain `.env` + `.env.example` (Zero tooling, works everywhere)

The 12-factor app convention. Commit `.env.example` with documented placeholder values; each developer copies it to `.env` (gitignored) and fills in their own values. No extra tooling required.

```bash
# .env.example — committed, placeholders only
CF_TUNNEL_TOKEN=your-cloudflare-tunnel-token-here
DEV_HOSTNAME=your-name-dev-machine
```

The script sources it at the top with a guard:

```bash
# setup-env.sh — load secrets
if [[ ! -f ".env" ]]; then
  echo "❌ No .env file found. Copy .env.example to .env and fill in your values." >&2
  exit 1
fi
# shellcheck source=/dev/null
set -a; source .env; set +a
```

**Tradeoffs:** No tooling required, universally understood. Secrets live in a plaintext file on disk protected only by filesystem permissions and `.gitignore`. The primary risk is accidental `git add -f`.

---

#### Option B: `dotenvx` — Encrypted `.env`, Safe to Commit ⭐ Recommended

[`dotenvx`](https://dotenvx.com) is the modern evolution of `dotenv`, built by the original author. It uses **ECIES + AES-256-GCM** to encrypt secret values in-place inside `.env`. The encrypted file can be safely committed — only the `.env.keys` file (the private key) must stay secret.

```
.env          ← encrypted values, safe to commit to git
.env.keys     ← private decryption key, gitignored
.env.example  ← plaintext placeholders, committed for onboarding docs
```

**One-time setup per project:**

```bash
# Install (cross-platform)
brew install dotenvx/brew/dotenvx        # macOS
curl -sfS https://dotenvx.sh | sh        # Linux / any Unix

# Write real values to .env as normal
cat > .env <<EOF
CF_TUNNEL_TOKEN=eyJhI...
DEV_HOSTNAME=james-dev-machine
EOF

# Encrypt in place — rewrites .env with ciphertext, generates .env.keys
dotenvx encrypt

# The encrypted .env is now safe to commit; .env.keys is auto-gitignored
git add .env
```

**Running the script:**

```bash
# dotenvx decrypts at runtime and injects vars into the child process
dotenvx run -- ./setup-env.sh

# Or via Makefile:
setup:
	dotenvx run -- ./setup-env.sh init
```

The private key in `.env.keys` is shared with teammates via a password manager or set as `DOTENV_PRIVATE_KEY` in CI/CD pipeline secrets. The repo itself contains no plaintext secrets.

**Why this is better than plain `.env`:**
- The encrypted `.env` is commit-safe — no `.gitignore` dependency as a security control
- Works identically on macOS, Linux, and Windows
- No cloud service required, fully offline
- `dotenvx` also reads plain `.env` files — zero migration friction if you start with Option A
- Key rotation is explicit: `dotenvx encrypt` re-encrypts cleanly
- CI/CD only needs `DOTENV_PRIVATE_KEY` set as a pipeline secret

---

#### `.gitignore` — Scoped to Build Artifacts and Keys

`.gitignore`'s job here is to exclude build artifacts and the decryption key — not to be the primary secret control:

```gitignore
# .gitignore

# Build artifacts — generated from template at runtime, never commit
dev-machine.bu
config.ign

# dotenvx private key — share out-of-band, never commit
.env.keys

# Plain .env approach: uncomment if not using dotenvx encryption
# .env
```

#### Comparison of Cross-OS Approaches

| Approach | Secrets on disk? | Safe to commit? | Extra tooling | CI-compatible? | Cross-OS? |
|---|---|---|---|---|---|
| **Plain `.env`** | Plaintext | ❌ No | None | ✅ Env injection | ✅ Yes |
| **`dotenvx` encrypted `.env`** ⭐ | Encrypted | ✅ Yes | `dotenvx` binary | ✅ Via `DOTENV_PRIVATE_KEY` | ✅ Yes |
| **`direnv` + `.envrc`** | Plaintext | ❌ No | `direnv` + shell hook | ⚠️ Unix only | ⚠️ Unix/WSL |
| **macOS Keychain** | Never on disk | N/A | None (built-in) | ❌ No | ❌ macOS only |
| **`sops` + `age`** | Encrypted | ✅ Yes | `sops` + `age` | ✅ Yes | ✅ Yes |

**Recommendation:** Use **`dotenvx`** for the cleanest cross-OS solution — it turns the encrypted `.env` into a committed artifact so the repo is fully self-contained and there's no gitignore risk. Start with **plain `.env`** if you want zero tooling friction and add `dotenvx encrypt` later without changing the workflow.

---

### 2. Two Divergent Codepaths That Are Never Reconciled

The script (`setup-env.sh`) configures the VM via SSH heredocs using a **credentials-file JSON** approach (per-tunnel UUID). The Butane template (`dev-machine.template.bu`) uses the simpler **`--token`** approach. They are never connected — the template is effectively dead code.

This means:
- What's documented in the template ≠ what actually runs
- The committed `config.ign` reflects the template, not the script's behavior
- Neither path can be independently tested or diffed in PRs

**Recommended fix:**

Pick one strategy and delete the other. The `--token` approach from the template is recommended — it's simpler, doesn't require local credential JSON files, and is the modern Cloudflare-recommended pattern. The script should **generate** the Butane file from the template rather than replacing it with SSH heredocs:

```bash
# In setup-env.sh, replace the SSH heredoc block with:
export CF_TUNNEL_TOKEN DEV_HOSTNAME
envsubst < dev-machine.template.bu > dev-machine.bu
butane --pretty --strict dev-machine.bu > config.ign
podman machine init --ignition-path config.ign --rootful "$DEV_HOSTNAME"
```

---

## ⚠️ High Priority Issues

### 3. Silent Failures — `set -e` Without Error Context

The script uses `set -e` but exits silently on failure with no indication of what line failed or why. Debugging a broken setup on a new developer's machine is nearly impossible.

**Recommended fix:**

```bash
# Replace:
set -e

# With:
set -euo pipefail
trap 'echo "❌ Error on line $LINENO (exit code: $?)" >&2' ERR
```

`pipefail` also catches failures hidden inside pipelines, such as the `cloudflared tunnel list | grep ... | awk` chain which currently fails silently if any segment errors.

---

### 4. No Idempotency — Every Run Destroys the VM

```bash
podman machine rm -f "$DEV_HOSTNAME" 2>/dev/null || true
podman machine init --rootful "$DEV_HOSTNAME"
```

Re-running the script to rotate a Cloudflare token, or to fix a misconfiguration, destroys Tailscale authentication state (requiring manual re-auth), all running containers, and any persisted developer data. This makes the script unsafe to run more than once.

**Recommended fix:**

Introduce sub-commands with scoped behavior:

```bash
./setup-env.sh init           # Full teardown + rebuild (first-time setup)
./setup-env.sh update-tunnel  # Rotate CF credentials only, no VM teardown
./setup-env.sh status         # Health check — print tunnel + tailscale state
./setup-env.sh teardown       # Explicit, deliberate VM destruction
```

Implement with a simple dispatcher at the top of the script:

```bash
COMMAND="${1:-init}"
case "$COMMAND" in
  init)          cmd_init ;;
  update-tunnel) cmd_update_tunnel ;;
  status)        cmd_status ;;
  teardown)      cmd_teardown ;;
  *) echo "Usage: $0 {init|update-tunnel|status|teardown}" >&2; exit 1 ;;
esac
```

---

### 5. Unpinned Container Image Tags (`latest`)

Both daemon services pull `latest`:

```
docker.io/tailscale/tailscale:latest
docker.io/cloudflare/cloudflared:latest
```

Two developers onboarding days apart may get different images. An upstream breaking release silently breaks onboarding for everyone, with no way to pin back to a known-good version.

**Recommended fix:**

Pin to specific versions and update deliberately:

```
docker.io/tailscale/tailscale:v1.80.3
docker.io/cloudflare/cloudflared:2025.4.0
```

Add a `# pinned: YYYY-MM-DD` comment next to each tag. A `make update-pins` target can automate checking for newer versions.

---

### 6. Fragile `awk`-based Tunnel UUID Parsing

```bash
TUNNEL_UUID=$(cloudflared tunnel list | grep "$TUNNEL_NAME" | awk '{print $1}')
```

The column format of `cloudflared tunnel list` is not a stable API contract. If the format changes or `grep` returns no match, `TUNNEL_UUID` is silently empty and the script continues in a broken state — creating config files with a blank UUID.

**Recommended fix:**

Use the structured JSON output:

```bash
TUNNEL_UUID=$(cloudflared tunnel list --output json \
  | jq -r ".[] | select(.name == \"$TUNNEL_NAME\") | .id")

if [[ -z "$TUNNEL_UUID" ]]; then
  echo "❌ Error: Could not find tunnel UUID for '$TUNNEL_NAME'" >&2
  exit 1
fi
```

This also eliminates the dependency on `awk` and makes the intent explicit. Requires `jq` (add to prerequisites check).

---

## 🏗️ Structural Improvements

### 7. Add a `Makefile` as the Standard Entry Point

A `Makefile` makes targets discoverable (`make help`), composable for CI pipelines, and serves as living documentation for operators:

```makefile
.PHONY: setup update-tunnel status teardown build-ignition lint help

SHELL := /bin/bash

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup: lint ## Full first-time setup
	./setup-env.sh init

update-tunnel: ## Rotate Cloudflare credentials without rebuilding VM
	./setup-env.sh update-tunnel

status: ## Check health of tunnel and Tailscale
	./setup-env.sh status

teardown: ## Destroy the VM (destructive)
	./setup-env.sh teardown

build-ignition: ## Compile template → dev-machine.bu → config.ign
	envsubst < dev-machine.template.bu > dev-machine.bu
	butane --pretty --strict dev-machine.bu > config.ign

lint: ## Run shellcheck + butane validation
	shellcheck setup-env.sh
	CF_TUNNEL_TOKEN=dummy DEV_HOSTNAME=test \
	  envsubst < dev-machine.template.bu | butane --strict --check
```

---

### 8. Consolidate to Pure Butane/Ignition for VM Configuration

The correct Fedora CoreOS pattern is to configure the VM at **boot time via Ignition**, not post-boot via SSH heredocs. SSH configuration injection is fragile, not reviewable as code, and can't be validated before runtime.

**Recommended flow:**

```
dev-machine.template.bu
        │  envsubst (CF_TUNNEL_TOKEN, DEV_HOSTNAME)
        ▼
dev-machine.bu          ← gitignored build artifact
        │  butane --strict
        ▼
config.ign              ← gitignored build artifact
        │  podman machine init --ignition-path config.ign
        ▼
  VM boots fully configured
  No SSH injection required
```

Benefits:
- Config is reviewable as a diff in PRs (via the template)
- `butane --strict --check` validates the template in CI before anyone runs it
- Reproducible: same inputs always produce the same VM state
- The entire SSH heredoc block in `setup-env.sh` (~70 lines) is deleted

---

### 9. Add a Prerequisite Check Function

The README lists prerequisites but the script never validates them programmatically. A new developer missing `jq` or `butane` gets a cryptic error mid-run rather than an actionable message upfront.

**Recommended fix:**

Add this function and call it first:

```bash
check_prerequisites() {
  local missing=0
  for cmd in podman cloudflared jq butane envsubst; do
    if ! command -v "$cmd" &>/dev/null; then
      echo "❌ Missing required tool: $cmd" >&2
      missing=1
    fi
  done
  if [[ "$missing" -eq 1 ]]; then
    echo ""
    echo "Install missing tools with Homebrew:"
    echo "  brew install podman cloudflared jq butane gettext"
    exit 1
  fi
}
```

---

### 10. Add GitHub Actions CI for Validation

The repo has no CI. Without it, there's no guarantee `dev-machine.template.bu` is valid Butane or that `setup-env.sh` has no obvious shell errors before a developer runs it.

**Recommended fix:**

```yaml
# .github/workflows/validate.yml
name: Validate

on:
  push:
    branches: [main]
  pull_request:

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: shellcheck
        run: shellcheck setup-env.sh

      - name: Validate Butane template
        run: |
          CF_TUNNEL_TOKEN=dummy DEV_HOSTNAME=ci-test \
            envsubst < dev-machine.template.bu | \
            docker run --rm -i quay.io/coreos/butane:release --strict
```

This catches template syntax regressions and shell script errors on every PR with no local tooling required.

---

## Priority Summary

| # | Status | Issue | Severity | Effort | Recommended Fix |
|---|---|---|---|---|---|
| 1 | ✅ Completed | Secrets baked into build artifacts | 🔴 Critical | Low | Handled securely via Go `.env` management and runtime ignition population |
| 2 | ✅ Completed | Two divergent, unconnected codepaths | 🔴 Critical | Medium | Solved by pure Butane template to Ignition JSON workflow |
| 3 | 🚫 Not Relevant | Silent error handling (`set -e`) | 🟠 High | Low | Handled automatically by Go explicit error returns and Cobra constraints |
| 4 | ✅ Completed | VM always destroyed (no idempotency) | 🟠 High | Medium | Implemented complete subcommands: `init`, `teardown`, `update-tunnel`, `status` |
| 5 | ⬜️ Pending | Unpinned `latest` image tags | 🟠 High | Low | Pin `tailscale` and `cloudflared` to specific version tags |
| 6 | ✅ Completed | Fragile `awk` UUID parsing | 🟠 High | Low | Mitigated entirely through Cloudflare's Go SDK and JSON API handling |
| 7 | ✅ Completed | No standard entry point | 🟡 Medium | Low | Unified `devx` CLI created using Cobra framework |
| 8 | ✅ Completed | Config via SSH heredocs instead of Ignition | 🟡 Medium | High | Consolidated mapping of Butane configuration locally over SSH |
| 9 | ✅ Completed | No prerequisite validation | 🟡 Medium | Low | Validation executes directly inside `devx init` |
| 10 | ✅ Completed | No CI | 🟡 Medium | Low | Implemented build infrastructure using `mage` (Magefile CI pipeline) |

---

## Recommended Implementation Order

1. **Foundation:** Gitignore build artifacts; add `.env.example`; `dotenvx encrypt` for commit-safe secrets (or plain `.env` if zero-tooling preferred) (#1)
2. **Quick wins:** Add `set -euo pipefail` + ERR trap, pin image tags, fix UUID parsing (#3, #5, #6)
3. **Core refactor:** Unify to the Butane/Ignition path and delete SSH heredoc block (#2 + #8 together — these pair naturally)
4. **Polish:** Add sub-commands, `Makefile`, prerequisite check, CI (#4, #7, #9, #10)
