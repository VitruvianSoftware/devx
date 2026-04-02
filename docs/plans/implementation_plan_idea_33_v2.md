# CLI Integration Test Harness (Idea #33)

This plan outlines the architecture for testing the `devx` CLI commands (`devx shell`, `devx scaffold`, etc.) comprehensively without requiring a real container runtime VM (Podman/Docker) to be active.

## Chosen Architecture: The Fake `$PATH` Runtime (Option B)

We will use the **Fake `$PATH` Runtime** interception trick to test our system. 
- Creates an `internal/testutil/fake_runtime.go`.
- This utility generates temporary fake `podman`, `docker`, and `cloudflared` executable scripts during test setup.
- It dynamically prepends this temporary directory to `$PATH` within the context of the test execution.
- When `devx` attempts to run `exec.Command("podman")`, it hits our fake bash script which logs the invocation to a JSON log file and returns predefined success responses.
- *Benefit:* Requires **zero** changes to the existing production code geometry. We only write the test harness and test files.

## Proposed Changes

### 1. `internal/testutil/fake_runtime.go`
- A utility `NewFakeRuntime(t *testing.T)` that provisions the `fake_bin` directory and writes the proxy shell scripts.
- Modifies `os.Setenv("PATH", ...)` safely with tear-down cleanup.
- Exposes `func (f *FakeRuntime) Requests() [][]string` to parse the serialized invocation logs, allowing tests to assert exactly what commands the CLI attempted to execute.

### 2. `cmd/shell_test.go`
Table-driven integration tests for `devx shell` ensuring:
- AI bridge injection logic maps the correct `-v` flags.
- `.env` files are correctly mapped as `-e` overrides.
- Container environment logic doesn't crash when executed against mocked runtimes.

### 3. `cmd/scaffold_test.go`
Table-driven integration tests for `devx scaffold` ensuring:
- The `--force` flag alters execution behavior properly (if it invokes external tooling).

## Verification Plan

1. I will implement the test harness natively.
2. I will execute standard Go tests: `go test ./cmd/... -v` to ensure the table-driven tests successfully intercept fake invocations.
3. I will provide the successful CLI test output as proof of coverage, cementing the new testing architecture.
