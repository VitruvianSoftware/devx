# Future Enhancements & IDEAS

This document tracks upcoming feature ideas, requests, and architectural plans for `devx`. 

When an idea is fully implemented and shipped, it is **migrated to `FEATURES.md`** to maintain an organized historical record of capabilities.

---

## Idea Template

To propose a new feature, copy the template below and add it to the **Proposed Ideas** section. Please ensure you increment the Idea Number sequentially (the last shipped feature was 28).

```markdown
### [Idea Number]. [Feature Title]
* **The Problem:** [Describe the workflow friction, missing capability, or developer pain point]
* **The Solution:** [Describe how `devx` will solve this seamlessly. Mention potential commands, UI (TUI/logs), flags, or architecture]
* **Key files:** [Optional: List core files or packages to be added or modified]
```

---

## Proposed Ideas





### 33. CLI Integration Test Harness
* **The Problem:** The `cmd/` layer — our most user-facing surface — has zero test coverage. Commands like `devx shell`, `devx scaffold`, and `devx cloud spawn` contain complex branching logic (env injection, idempotency guards, mount detection) that is entirely untested, creating a silent regression risk on every PR.
* **The Solution:** Build a dedicated integration test harness for the `cmd/` package. Use a fake/mock container runtime backend to allow tests to run without a real Podman VM. Write table-driven test cases covering the most critical code paths: AI bridge injection logic, agent config mount discovery, `.env` vs. vault override precedence, and `--force` flag behavior on scaffold.
* **Key files:** `cmd/shell_test.go`, `cmd/scaffold_test.go`, `internal/ai/bridge_test.go`, `internal/testutil/fake_runtime.go`
