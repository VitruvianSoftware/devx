# Idea 33: CLI Integration Test Harness

This implementation plan outlines a comprehensive, robust integration test harness for the `cmd/` layer to guarantee business-critical CLI paths (like `devx shell` and `devx scaffold`) do not silently regress. 

## User Review Required

> [!IMPORTANT]
> **Mocking Boundary:** Currently, `devx` commands heavily invoke `exec.Command` directly to drive `podman` and `docker`. The standard Go way to mock this without massive architectural interface refactoring (which would disrupt dozens of existing files) is to replace `exec.Command` calls with a package-scoped `var execCommand = exec.Command` variable, allowing tests to temporarily swap it out with a fake recorder. Is this lightweight interception pattern acceptable to you, or would you prefer a heavy refactor where we inject an `Executor` interface into every command?

> [!NOTE]
> **Test Coverage Focus:** I plan to immediately test:
> 1. `devx shell`: Verifying it properly discovers AI ports, intercepts `.env` versus remote vaults, and accurately constructs the massive `-e` and `-v` mount arrays.
> 2. `devx scaffold`: Verifying the new `Result` tracking and idempotency `--force` logic skips/overwrites correctly using an isolated `/tmp` workspace.
> Does this align with your immediate priorities, or are there other commands you want guaranteed in this initial test pass?

## Proposed Changes

---

### 1. The Interception Point
#### [MODIFY] `cmd/shell.go`, `cmd/scaffold.go`, `cmd/cloud_spawn.go`
- Replace direct `exec.Command(...)` calls with a package-scoped `execCommand(...)` function variable.
- By default, `var execCommand = exec.Command`.

### 2. The Fake Runtime Harness
#### [NEW] `internal/testutil/exec_mock.go`
Implementing the standard Go `TestHelperProcess` pattern to cleanly intercept and stub OS/Subprocess executions.
- **Recording:** Captures the exact arguments passed to `podman` or `docker`.
- **Stubbing Outputs:** Allows our tests to fake the output of `podman ps` or `gcloud auth` by mapping command sub-strings to mock raw string outputs.

### 3. Key Integration Tests

#### [NEW] `cmd/shell_test.go`
- Uses table-driven scenarios to run the `shellCmd.RunE` endpoint with fake contexts.
- **Test cases will assert:**
  - AI Bridge: If host LLM is present, ensure `OPENAI_API_BASE` is injected in the recorded podman command.
  - User Overrides: If `.env` forces `OPENAI_API_BASE`, ensure the AI bridge is skipped.
  - Agent Mounts: Verify `-v ~/.claude.json:...` appears when the file physically exists on the test mock filesystem.

#### [NEW] `cmd/scaffold_test.go`
- Employs a temporary underlying filesystem `t.TempDir()`.
- **Test cases will assert:**
  - Safe Default: Scaffold templates generate cleanly. Re-running the scaffold skips files that exist and returns `Skipped=N`.
  - Force Override: Passing `--force` explicitly overwrites modified template files.

#### [NEW] `internal/ai/bridge_test.go`
- A lightweight unit test simulating open vs closed ports using real `net.Listen` on ephemeral TCP sockets, ensuring `DiscoverHostLLMs` only injects configuration if an engine is actively responding.

## Verification Plan

### Automated Tests
1. Run `go test ./cmd/... ./internal/ai/... -v`
2. Assert 0 execution errors.
3. Assert minimum acceptable branch coverage `> 70%` specifically on `shell.go` and `scaffold.go`.

### Manual CI Verification
- Commit the test suite and confirm that GitHub Actions executes the tests properly, proving the `TestHelperProcess` functions execute correctly in a remote Ubuntu runner natively, preventing regressions on PR #33 and beyond.
