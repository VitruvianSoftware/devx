# Idea 40: `devx agent ship` (Deterministic Agent Pipeline Guardrails)

You've hit on the exact missing puzzle piece for **Agentic Development** (AX). We are currently relying on LLMs "remembering" to follow markdown checklists, which is as fundamentally brittle as asking human developers to manually run linters before pushing.

By baking the automation loop directly into the `devx` binary, we flip the ecosystem. We force agents to interact with a strictly typed API boundary that **mechanically refuses to let them succeed until verified.** 

## Design Principle: Proactive Mitigation
Gone are the days of frustrating tools that print a wall of errors and dump manual instructions for the developer to follow. `devx` operates on the principle of **Proactive Mitigation**. We do not assume things are configured, setup, or initialized. If an error is detected or a tool is missing, `devx` must proactively halt, explain the gap, and immediately offer an automated mitigation (e.g., `"You are missing X. Install it now? [Y/n]"`).

## Proposed Feature: `devx agent ship` & Strict Git Hooks

We introduce a new command, `devx agent ship` (or `devx agent sync`), and enforce its usage across the repository by intercepting the standard Git workflow using local Git Hooks (`.git/hooks/pre-push`).

### 1. The Interception Hook (The Forcing Function)
Because agents (and humans) inherently rely on muscle memory and might accidentally run `git push`, we cannot trust them to "remember" to use the new tooling.
- During `devx init`, we automatically install a `.git/hooks/pre-push` hook.
- When an agent blindly attempts to run `git push`, the hook triggers `devx hook pre-push`.
- This hook explicitly blocks the push (returning Exit Code 1) and prints an unmissable warning: *"Direct `git push` is blocked. AI Agents MUST use `devx agent ship` to guarantee CI Pipeline validation."*
- **Human Optionality:** Strict pipeline blocking is intrinsically optional for humans, who can simply bypass the hook using standard git behavior (`git push --no-verify`). AI agents, however, are trapped and mathematically forced to route through `devx agent ship`.

### 2. Mandatory Pre-Flight Phase
When the agent complies and runs `devx agent ship`, it automatically profiles the repository (looking at `go.mod`, `package.json`, etc.) and executes the local pre-flight checks inline.
* **Agentic value:** The agent just runs `devx agent ship -m "commit"`. It doesn't need to know the repo's specific test harness.

### 3. Documentation Validation
Because `devx` knows what changed (via `git diff`), it can programmatically detect if a `.go` API endpoint was modified without a corresponding update in `docs/`.
* **Agentic value:** Built-in Definition of Done.

### 4. Synchronous CI Polling (The "Wait Trap")
After running the pre-flight checks, `devx agent ship` executes the push internally (using `--no-verify` to bypass our own hook). Critically, **it does not exit**. 
Using the GitHub API, it attaches to the pipeline and streams status directly to standard out.
* **Agentic value:** The `run_command` tool in the AI's toolkit hangs intentionally. The agent enters a suspended state, completely incapable of calling a task "Done" until the remote CI server returns a green artifact.

### 5. Deterministic Error Trapping & Formatting
When an agent currently tries to debug CI, they run cryptic bash pipes (`gh run view --log-failed ...`). `devx` will natively handle parsing CI responses.
If the pipeline fails, `devx agent ship` returns a specific predictive exit code (e.g., `ExitCodeCIFailed`) and outputs a hyper-condensed, agent-friendly JSON payload containing *just* the failure logs, stripping out all the verbose Node/Go installation noise.

## Expanding `devx doctor` (Autonomous Healing)

Because we operate on the **Proactive Mitigation** principle, `devx doctor` is no longer strictly an opt-in command. It will run autonomously in the background if `devx` detects execution within a stale or un-initialized repository. Instead of letting `devx up` or `devx agent ship` crash confusingly, the CLI will transparently run `doctor`, trap the faults, and offer to interactively heal the environment.

We will expand `doctor` to audit these new integration layers:
1. **Hook Diagnosis:** `devx doctor` will audit `.git/hooks/` to verify the `pre-push` automation isn't accidentally removed or misconfigured.
2. **Agent Toolkit Check:** Verifying that the `.agent`, `.claude`, and `.cursor` skills templates are correctly bootstrapped and synchronized.
3. **Workspace Schema Validation:** Adding a strict `devx.yaml` linter into `doctor` to catch bad configuration keys interactively before they crash the DAG orchestrator.

## Verification Plan: Dogfooding

> [!IMPORTANT]
> This feature **must be shipped using itself.** After implementation is complete, I will NOT fall back to `git push` or any manual merge workflow. I will run `devx agent ship` to commit, push, and synchronously wait for the CI pipeline to return green — proving the tool works end-to-end from inside the exact agentic context it was designed to govern.

This serves as both the ultimate integration test and a live proof-of-concept:
1. The `pre-push` hook must block me if I accidentally try `git push`.
2. `devx agent ship` must run local pre-flight checks (tests, lint, build).
3. `devx agent ship` must push to GitHub, create the PR, merge it, and then hold my terminal hostage until the CI pipeline completes.
4. I must report the final pipeline status (pass/fail) back to you directly from the tool's stdout — not from a separate `gh run list` invocation.
