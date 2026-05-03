# Implementation Plan: Peer-to-Peer State Replication (Idea 56)

**Date:** 2026-05-03
**Idea/Issue Reference:** Idea 56 (`devx state share` & `devx state attach`)

---

## 1. Deep Research & Context

* **Objective:** Enable developers to bundle their running container states, database volume snapshots, environment metadata, and container image SHAs into a single encrypted, portable artifact. A developer runs `devx state share` to produce a unique ID; a teammate runs `devx state attach <ID>` to instantly boot the exact same broken environment — eliminating "works on my machine" debugging friction.

* **Sibling Implementations Studied:**
  * [cmd/state.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state.go) — Parent command (`GroupID: "orchestration"`). New subcommands (`share`, `attach`) will be registered here via `stateCmd.AddCommand()`.
  * [cmd/state_checkpoint.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_checkpoint.go) — Closest sibling. Uses `getFullProvider()`, `DryRun`/`NonInteractive` guards, `prov.VM.Name()` for provider string, then calls `state.CreateCheckpoint(providerName, name, rt)`. Our `share` command will follow this exact pattern.
  * [cmd/state_restore.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_restore.go) — Closest sibling for `attach`. Uses interactive confirm with `y/N` pattern, calls `state.RestoreCheckpoint(prov.VM.Name(), name, prov.Runtime)`. Our `attach` command will follow this pattern.
  * [cmd/state_dump.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_dump.go) — Shows the `--json` / `--file` flag pattern for outputting structured state data.
  * [cmd/state_list.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_list.go) / [cmd/state_rm.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_rm.go) — Shows `tabwriter` list pattern and destructive confirmation pattern.
  * [internal/state/checkpoint.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/checkpoint.go) — The engine. Key findings:
    * `CreateCheckpoint` uses `exec.Command("podman", "container", "checkpoint", id, "-e", archivePath, "--keep")` — this runs **on the host** via `podman` CLI, and Podman Machine transparently handles the VM filesystem bridge via `--export`. The archive lands at `~/.devx/checkpoints/<name>/<container>.tar.gz` on the host.
    * `RestoreCheckpoint` similarly uses `exec.Command("podman", "container", "restore", "-i", arch)`.
    * Provider guard: `if providerName != "podman"` — string comparison, NOT using `rt.SupportsCheckpoint()`.
  * [internal/database/snapshot.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/database/snapshot.go) — DB volume snapshots. Key findings:
    * `CreateSnapshot(rt, engine, name)` uses `rt.CommandContext()` for both Podman (`volume export`) and Docker (Alpine helper container). Archives land at `~/.devx/snapshots/<engine>/<name>.tar` with a sibling `.json` metadata file.
    * Supports both Podman and Docker/nerdctl (unlike CRIU checkpoints which are Podman-only).
  * [internal/devxerr/error.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/devxerr/error.go) — Exit code registry. Current highest code: `72` (Bridge). New codes for state replication will start at `80`.
  * [cmd/devxconfig.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/devxconfig.go) — The `DevxConfig` struct (line 232). New `State` field will be added here.
  * [internal/provider/provider.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/provider/provider.go) — `ContainerRuntime` interface includes `SupportsCheckpoint() bool` (line 104). Only `PodmanRuntime` returns `true` (line 138). Docker/nerdctl return `false`.

* **Gap Analysis:**
  1. **`SupportsCheckpoint()` is unused.** `checkpoint.go` uses `providerName != "podman"` string check instead of `rt.SupportsCheckpoint()`. Our code will use the same string check for consistency with the existing pattern, but we should call this out as tech debt.
  2. **No new Go dependencies needed for crypto.** Go's standard library `crypto/aes`, `crypto/cipher`, `crypto/rand`, `crypto/sha256` provide everything for AES-256-GCM. `golang.org/x/crypto/pbkdf2` is NOT in `go.mod` — we will use `crypto/sha256` based HKDF or argon2 from stdlib-compatible approaches, OR we can add `golang.org/x/crypto` as a dependency (it's a Go sub-repo, standard practice).
  3. **Checkpoint is Podman-only; DB snapshot is cross-runtime.** `devx state share` will have two modes: full (CRIU + volumes, Podman only) and partial (volumes only, any runtime). This degradation must be communicated clearly.

## 2. Design Decisions

### Design Principles Alignment

* [x] **One CLI, everything** — Extends the existing `devx state` command hierarchy. `share` and `attach` are natural siblings of `checkpoint`/`restore`/`dump`.
* [x] **Convention over configuration** — Bring-Your-Own-Bucket (S3/GCS) is strictly enforced. It can be configured globally in `devx.yaml` or per-command via CLI flag. The encryption key is auto-generated.
* [x] **Transparency** — Before uploading, `share` will print a full manifest: number of containers, number of DB volumes, total bundle size, and destination bucket. Before restoring, `attach` will print what will be stopped/overwritten.
* [x] **Idempotency** — `share` creates a temporary checkpoint name (`_share_<timestamp>`), bundles it, then deletes the temp checkpoint. Running twice produces two independent bundles. `attach` is destructive (overwrites local state) but shows interactive confirmation unless `-y`.
* [x] **AI-native** — Both commands support `--json` (returns structured `{"id": "...", "containers": N, "size": "..."}`) and `--dry-run` (simulates without uploading/restoring). Deterministic exit codes registered in `devxerr`.
* [x] **CLI + YAML parity** — The relay destination can be configured via `--relay` CLI flag OR `state.relay` in `devx.yaml`. CLI flag wins (consistent with provider cascade).
* [ ] **Optimized Inner Loop** — Does not directly apply (this is a debugging/collaboration tool, not an inner-loop speed feature).
* [x] **Client-Driven Architecture** — No server component. Encryption happens entirely on-device. S3/GCS uses the user's own credentials. There is no intermediate third-party service storing the encrypted blobs.
* [x] **Absolute Portability** — The crypto and bundling code is pure Go stdlib. The relay HTTP client is stdlib `net/http`. CRIU checkpoints are Podman-only (explicit error on Docker/Lima/Colima), but DB-only snapshots work on all runtimes. macOS/Linux, Intel/ARM all supported.

### Non-Obvious Choices

1. **Why enforce Bring-Your-Own-Bucket (S3/GCS) instead of a public relay?** While a public ephemeral relay (like `transfer.sh`) provides a zero-config "happy path", relying on a third-party service for potentially sensitive state transfers is a security and reliability risk for enterprise teams. Enforcing S3/GCS ensures that all artifacts remain within the organization's own infrastructure perimeter.

2. **Why `golang.org/x/crypto` for key derivation?** Go stdlib has AES-GCM but no PBKDF2/Argon2. `golang.org/x/crypto` is the canonical Go sub-repository (maintained by the Go team) and provides `pbkdf2.Key()`. This is a standard dependency for any Go project doing password-based encryption. Alternative: raw SHA-256 hashing — rejected because it lacks proper key stretching.

3. **Why a 4-word passphrase instead of a random hex string?** The ID must be communicated human-to-human (Slack, verbal). `fast-blue-rabbit-dawn` is dramatically easier to dictate over a call than `a3f2c9b1e4d7`. The passphrase doubles as the AES key derivation input, so it IS the security — longer passphrases = stronger keys.

4. **Why NOT use `rt.SupportsCheckpoint()` for the guard?** The existing `checkpoint.go` code uses `providerName != "podman"` string comparison. Changing this would be a refactor of existing code, not part of this feature. We will follow the established pattern for consistency and note it as future tech debt.

5. **Why two modes (full vs DB-only)?** CRIU is Podman-only, but database volume export works on all runtimes via the existing `database.CreateSnapshot()` helper container pattern. Rather than blocking the entire feature for non-Podman users, we offer a "DB-only" mode that shares database state without container memory. This matches the existing degradation pattern from Feature 5 (provider abstraction).

## 3. Proposed Changes

### `internal/state/` — Replication Engine (dependencies first)

* **`[NEW]` `internal/state/replication.go`:**
  * `BundleManifest` struct: records container checkpoint paths, DB snapshot paths, image SHAs (`podman inspect --format '{{.Image}}'`), and `devx.yaml` hash.
  * `BundleState(rt, checkpointName string, dbSnapshots []database.SnapshotMeta) (*Bundle, error)`: Creates `~/.devx/share/<uuid>/` containing checkpoint archives + DB snapshot tars + `manifest.json`. Tars and gzips everything into `bundle.tar.gz` (using `archive/tar` and `compress/gzip`).
  * `UnbundleState(archivePath string) (*BundleManifest, error)`: Extracts a bundle to a temp dir, returns parsed manifest.
  * `CleanupBundle(bundleDir string)`: Removes temporary bundle artifacts.

* **`[NEW]` `internal/state/crypto.go`:**
  * `GeneratePassphrase() string`: Generates a 4-word passphrase from a built-in wordlist (BIP-39 subset, ~2048 words = ~44 bits entropy per word, 4 words = ~176 bits).
  * `EncryptFile(inputPath, outputPath, passphrase string) error`: PBKDF2 key derivation (SHA-256, 600k iterations) → AES-256-GCM encrypt. Prepends salt + nonce to output file.
  * `DecryptFile(inputPath, outputPath, passphrase string) error`: Reads salt + nonce, derives key, AES-256-GCM decrypt.

* **`[NEW]` `internal/state/relay.go`:**
  * `UploadToS3(filePath, s3URI string) error`: Shells out to `aws s3 cp` (checked via `exec.LookPath("aws")`).
  * `UploadToGCS(filePath, gsURI string) error`: Shells out to `gcloud storage cp` (checked via `exec.LookPath("gcloud")`).
  * `DownloadFromS3/GCS` counterparts.
  * `ParseRelay(relayConfig string) (backend string, uri string)`: Parses `s3://...` vs `gs://...` and validates it. (Will include TODO breadcrumbs for future HTTP endpoint support).

### `cmd/` — CLI Layer

* **`[NEW]` `cmd/state_share.go`:**
  * `Use: "share"`, `GroupID` inherited from `stateCmd` (orchestration).
  * `Short: "Bundle and share your running environment state with a teammate"`
  * `Long`: Explains full vs DB-only modes, encryption, relay, and S3/GCS support.
  * `Example`: `devx state share`, `devx state share --relay s3://my-bucket/state`, `devx state share --db-only`
  * Flags: `--relay` (string, override relay destination), `--db-only` (bool, skip CRIU, share DB volumes only).
  * Flow:
    1. `getFullProvider()`
    2. Check `rt.Name()` — if not `podman` and `--db-only` not set, auto-enable `--db-only` with a warning.
    3. If full mode: call `state.CreateCheckpoint(prov.VM.Name(), "_share_<timestamp>", rt)` to create temp checkpoint.
    4. Discover devx-managed DB volumes and call `database.CreateSnapshot(rt, engine, "_share_<timestamp>")` for each.
    5. Call `BundleState()` to tar everything.
    6. Call `GeneratePassphrase()`, then `EncryptFile()`.
    7. Resolve relay (flag → `devx.yaml state.relay`). Fail-fast if no bucket is configured.
    8. Call `UploadToS3/GCS`.
    9. Cleanup temp checkpoint and snapshots.
    10. Print the ID: `<token>:<passphrase>` or `<backend>:<path>:<passphrase>`.
    11. `--json` output: `{"id": "...", "containers": N, "databases": N, "size_bytes": N, "relay": "...", "mode": "full|db-only"}`.
    12. `--dry-run`: Print manifest + estimated size without uploading.

* **`[NEW]` `cmd/state_attach.go`:**
  * `Use: "attach <ID>"`, `Args: cobra.ExactArgs(1)`.
  * `Short: "Download and restore a shared environment state from a teammate"`
  * Flow:
    1. Parse the ID into `(backend, token, passphrase)`. Fail-fast with `devxerr` if malformed.
    2. Download the encrypted bundle.
    3. Decrypt with passphrase. Fail-fast with clear error on wrong passphrase.
    4. Unbundle and read manifest.
    5. Interactive confirmation showing what will be destroyed (unless `-y`).
    6. If CRIU archives present: call `state.RestoreCheckpoint()`.
    7. If DB snapshots present: call `database.RestoreSnapshot()` for each engine.
    8. Cleanup temp files.
    9. Print success summary.

### `cmd/devxconfig.go` — Schema Change

* **`[MODIFY]` [cmd/devxconfig.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/devxconfig.go#L231-L246):**
  * Add new struct and field to `DevxConfig`:
    ```go
    // DevxConfigState defines state sharing/replication settings.
    type DevxConfigState struct {
        Relay string `yaml:"relay"` // Upload destination: s3://... or gs://...
    }
    ```
  * Add to `DevxConfig` struct (after line 245):
    ```go
    State *DevxConfigState `yaml:"state"` // State replication settings (Idea 56)
    ```

### `internal/devxerr/error.go` — New Exit Codes

* **`[MODIFY]` [internal/devxerr/error.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/devxerr/error.go#L25-L52):**
  * Add new constants after Bridge section (starting at 80):
    ```go
    // State Replication Errors (Idea 56)
    CodeStateShareNoContainers  = 80 // No running devx containers to share
    CodeStateShareUploadFailed  = 81 // Failed to upload bundle to relay/bucket
    CodeStateAttachInvalidID    = 82 // Malformed share ID
    CodeStateAttachDownloadFail = 83 // Failed to download bundle
    CodeStateAttachDecryptFail  = 84 // Wrong passphrase or corrupted bundle
    CodeStateAttachRestoreFail  = 85 // Checkpoint or snapshot restore failed
    ```

### `cmd/state.go` — Parent Description Update

* **`[MODIFY]` [cmd/state.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state.go#L27-L35):**
  * Update `Long` description to mention `share` and `attach`:
    ```go
    Long: `The state command hierarchy manages the macro state of the entire devx environment.

    You can generate full diagnostic dumps, checkpoint/restore the system snapshot using CRIU,
    and share your complete running environment with teammates via 'devx state share'.`,
    ```

### Architecture & Runtime Compliance Checklist

* [x] **Provider Abstraction:** Uses `getFullProvider()` and `prov.Runtime` for all container operations. CRIU path uses `exec.Command("podman", ...)` directly, matching the existing `checkpoint.go` pattern. DB snapshot path uses `rt.CommandContext()` via `database.CreateSnapshot()`, supporting all runtimes.
* [x] **Config Cascade:** `--relay` flag → `devx.yaml state.relay`. Follows the same precedence as `--provider` flag → `devx.yaml provider` → auto-detect. Fails if no relay is configured.
* [x] **Global Flags:** `--dry-run` prints manifest + estimated size without uploading or modifying state. `-y` bypasses the interactive confirmation on `attach`. `--json` returns structured output from both commands. `--provider` is inherited from root and used by `getFullProvider()`.
* [x] **Error Handling Strategy:** Fail-fast with deterministic exit codes. Malformed ID → `CodeStateAttachInvalidID` (82). Wrong passphrase → `CodeStateAttachDecryptFail` (84). Network failure → `CodeStateShareUploadFailed` (81). All temporary artifacts are cleaned up on failure via `defer`.
* [x] **Environment & Context:** All commands execute on the **host**. Encryption/decryption is pure Go running on the host. CRIU `checkpoint --export` and `container restore -i` run on the host via `podman` CLI — Podman Machine handles the VM bridge transparently (same as existing `checkpoint.go`). DB volume export uses `rt.CommandContext()` which routes through the correct runtime wrapper. Upload/download is stdlib `net/http` or shelling out to `aws`/`gcloud` CLIs on the host.
* [x] **Pre-push Hook / `devx audit`:** No impact. This feature does not change exit codes, container execution paths, or scanning behavior for the pre-push guardrail.

## 4. Configuration & Schema Changes

* [ ] **`devx.yaml` keys:** Add optional `state` block:
  ```yaml
  state:
    relay: "s3://my-company-devx-state/checkpoints"  # or gs://...
  ```
* [ ] **Environment variables:** None new. S3/GCS auth uses existing `AWS_*` / `GOOGLE_APPLICATION_CREDENTIALS` env vars from the user's environment.
* [ ] **CLI flags:**
  * `devx state share --relay <URI>`: Override relay destination (takes precedence over `devx.yaml`).
  * `devx state share --db-only`: Skip CRIU checkpoint, share only database volumes. Auto-enabled when runtime is not Podman.
* [ ] **New Go dependency:** `golang.org/x/crypto` for `pbkdf2.Key()`. This is a Go sub-repository maintained by the Go team — standard practice.

## 5. Documentation Ecosystem (Mandatory)

### Official Docs & CLI
* [ ] **Official Docs (Vitepress):** Create `docs/guide/state-replication.md`. Cover: what gets bundled, encryption model (AES-256-GCM, PBKDF2), S3/GCS setup, full vs db-only mode, security considerations. Wire into `docs/.vitepress/config.mjs` sidebar under "Advanced" section.
* [ ] **CLI Help Text:** Add `Use`, `Short`, `Long`, and `Example` fields in `cmd/state_share.go` and `cmd/state_attach.go`. Ensure `GroupID` is inherited from `stateCmd` (orchestration).
* [ ] **Environment Health (`devx doctor`):** Add a feature readiness entry for `devx state share` that checks: at least one runtime installed + `aws` or `gcloud` if S3/GCS relay is configured in `devx.yaml`. Update [cmd/doctor.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/doctor.go#L251-L292) `computeFeatureReadiness()`.
* [ ] **Main README:** Add `devx state share` / `devx state attach` to the CLI reference table.
* [ ] **Feature Trackers:** Migrate Idea 56 from `IDEAS.md` to `FEATURES.md` with full write-up.
* [ ] **CHANGELOG:** Auto-generated by release-please. Verify commit message: `feat(state): implement peer-to-peer state replication (Idea 56)`.

### Agent Skills Templates (Mandatory Audit)

> [!CAUTION]
> The existing skill files do NOT currently document any `devx state` subcommands (verified: grep returned no matches). We must add the full `state share` and `state attach` documentation.

* [ ] `.agents/skills/devx/SKILL.md` — Add `devx state share` and `devx state attach` command reference with exit codes 80-85.
* [ ] `.agents/skills/platform-engineer/SKILL.md` — Add `state share/attach` to the troubleshooting workflow.
* [ ] `.github/skills/devx/SKILL.md` — Mirror of `.agents/skills/devx/SKILL.md`.
* [ ] `.github/skills/platform-engineer/SKILL.md` — Mirror of `.agents/skills/platform-engineer/SKILL.md`.

### Example Configs & Templates
* [ ] **`devx.yaml.example`:** Add commented `state:` block with relay examples (after the bridge section, around line 326).
* [ ] **`CONTRIBUTING.md`:** Add `internal/state/replication.go`, `crypto.go`, `relay.go` to the project structure diagram.

## 6. Verification Plan

### Automated Tests
* [ ] `internal/state/crypto_test.go` — Round-trip test: generate passphrase → encrypt file → decrypt file → verify content matches. Edge case: empty file, large file (>100MB simulated).
* [ ] `internal/state/relay_test.go` — Unit test `ParseRelay()` for `s3://`, `gs://`, and malformed inputs.
* [ ] `internal/state/replication_test.go` — Test `BundleManifest` serialization/deserialization.
* [ ] Run linters: `go vet ./...` / `staticcheck ./...`
* [ ] License headers: `mage licensecheck`
* [ ] Build verification: `go build ./...`
* [ ] Cross-platform CI: All 7 GitHub Actions checks must pass (Lint, Test, Validate Butane, Build ×4 platforms)

### Manual / Edge Case Verification
* [ ] **Edge Case 1:** Provider is Docker Desktop, `--db-only` not passed → *Expected: Auto-enables `--db-only` with warning: "CRIU checkpoints require Podman. Sharing database state only."*
* [ ] **Edge Case 2:** No running devx containers AND no devx databases → *Expected: Fail-fast with `CodeStateShareNoContainers` (80) and message: "Nothing to share — no running containers or databases found."*
* [ ] **Edge Case 3:** Malformed ID passed to `attach` (e.g., `"foo"`, missing passphrase) → *Expected: Fail-fast with `CodeStateAttachInvalidID` (82) before any network request.*
* [ ] **Edge Case 4:** Correct token but wrong passphrase → *Expected: Download succeeds, decrypt fails with `CodeStateAttachDecryptFail` (84) and message: "Decryption failed — check your share ID. The passphrase may be incorrect."*
* [ ] **Edge Case 5:** Network failure during upload → *Expected: Temporary bundle is cleaned up via `defer`, error reported with `CodeStateShareUploadFailed` (81).*
* [ ] **Edge Case 6:** Run `share` with `--dry-run` → *Expected: Prints manifest (N containers, N databases, estimated size), relay destination, and generated passphrase. No checkpoint created, no upload performed.*
* [ ] **Edge Case 7:** Run `share` with `--json` → *Expected: Valid JSON output with `id`, `containers`, `databases`, `size_bytes`, `relay`, `mode` fields.*
* [ ] **Edge Case 8:** S3 relay configured but `aws` CLI not installed → *Expected: Fail-fast with actionable error: "S3 relay configured but 'aws' CLI not found. Install it with: brew install awscli"*
* [ ] **E2E smoke test:** Full round-trip on Podman: `devx state share` → copy ID → `devx state attach <ID>` on a clean environment → verify containers are running.

## 7. Self-Review Checklist

- [x] I have **read every file I plan to modify** — `cmd/state.go` (40 lines), `cmd/devxconfig.go` (619 lines, full `DevxConfig` struct at L232-L246), `internal/devxerr/error.go` (84 lines), `devx.yaml.example` (326 lines).
- [x] I have **cross-referenced every function call** — `state.CreateCheckpoint(providerName string, name string, rt provider.ContainerRuntime)` at `checkpoint.go:48`, `state.RestoreCheckpoint(providerName string, name string, rt provider.ContainerRuntime)` at `checkpoint.go:120`, `database.CreateSnapshot(rt provider.ContainerRuntime, engine string, snapshotName string)` at `snapshot.go:66`, `database.RestoreSnapshot(rt provider.ContainerRuntime, engine string, snapshotName string)` at `snapshot.go:127`.
- [x] I have **not omitted documentation updates** — Vitepress page, README, doctor, CHANGELOG, all 4 agent skill files, `devx.yaml.example`, `CONTRIBUTING.md` all listed.
- [x] My verification plan **tests edge cases** — 8 edge cases covering wrong provider, empty state, malformed ID, wrong passphrase, network failure, dry-run, JSON output, and missing CLI tools.
- [x] I have **not assumed default values** — Checked `CheckpointsDir()` respects `DEVX_CHECKPOINT_DIR` env override, `SnapshotDir()` respects `DEVX_SNAPSHOT_DIR`, global flags defined in `cmd/root.go:37-40`.
- [x] I have audited all **4 agent skill template files** — Verified none currently mention `devx state` commands (grep confirmed no matches in `.agents/skills/devx/SKILL.md`).
- [x] I have verified that the project/task status (**IDEAS.md → FEATURES.md**) will be updated.


