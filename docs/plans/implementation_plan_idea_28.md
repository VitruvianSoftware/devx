# Idea 28: Zero-Friction Local AI Bridge

This plan implements a lightweight Host-to-Container AI Bridge. It abandons heavy containerized inference (which kills performance via CPU bottlenecks on macOS), instead seamlessly routing the isolated `devx shell` environment back to the developer's native host AI muscle (Ollama, LM Studio) and porting their existing AI CLI agent identities (Claude Code, Opencode, Gemini, Codex) directly into the workspace.

## User Review Required

> [!NOTE]
> **Advanced Agent Gaps:** Based on your request for deep research into agentic gaps, I've proposed three advanced mount ideas in **Section 3: Bridging Agentic Workflow Gaps**. Take a look and let me know if those (like the Podman Socket pass-through for agents that want to spin up their own containers) are worth including in this release.

## Proposed Changes

---

### 1. Transparent (But Optional) Host API Bridging

We will modify `cmd/shell.go` (or extract to `internal/ai/bridge.go`) to automatically detect and securely bridge native host APIs into the container SDKs, with explicit developer consent.

- **Auto-detect Host Instances:** During `devx shell` launch, do a fast TCP dial check on the host for standard AI ports (11434 for Ollama, 1234 for LM Studio).
- **Environment Variable Targeting:** We will inject a suite of variables designed to override the default cloud endpoints for the major tools so they hit the local LLM instead:
  - `OPENAI_API_BASE=http://host.containers.internal:11434/v1` (Standard generic endpoint used by Codex, Opencode, and many open-source agents)
  - `ANTHROPIC_BASE_URL=http://host.containers.internal:11434` (To intercept Claude Code or Anthropic SDKs if an OpenAI-to-Anthropic adapter is running)
  - `GEMINI_API_BASE=http://host.containers.internal:11434` (For Gemini-CLI overriding)
  - `OLLAMA_HOST=http://host.containers.internal:11434`
- **Developer Consent (The Override Warning):** 
  - If a local engine is detected, `devx` will check if the user has already hardcoded `OPENAI_API_BASE` in their `.env`. If they have, it respects the `.env` fully (transparent).
  - If they have *not*, it will print a prominent informative message indicating that agentic tools have been overridden to use the local LLM to prevent accidental API bills, and instruct them how to disable it via `devx.yaml`:
  ```text
  🤖 Local AI Detected (Ollama).
  ↳ Injected OPENAI_API_BASE / ANTHROPIC_BASE_URL to route local agents (claude, opencode, codex) 
    to your host GPU. To disable this override, set 'ai.bridge: false' in devx.yaml.
  ```

### 2. Agent Identity Mounting (The Identity Fix)

When dropping into a devcontainer, developers shouldn't have to re-login to `claude` or `opencode` or `gemini-cli`. We will auto-detect and mount the authentication states and configuration dotfiles for the big four:

| Agent Tool | Host Path | Container Path | Purpose |
|------------|----------|----------------|---------|
| **Opencode** | `~/.config/opencode` | `~/.config/opencode` | Gemini CLI integration |
| **Claude Code** | `~/.claude.json` / `~/.config/claude` | `~/.claude.json` / `...` | Anthropic CLI integration |
| **Gemini CLI** | `~/.config/gemini-cli` | `~/.config/gemini-cli` | Google API integration |
| **Codex / OpenAI** | `~/.codex` or `~/.openai` | `~/.codex` | Codex CLI integration |

If the directory exists on the host Mac, it is mounted read-only (or read/write if the token needs refreshing) into the exact same location in the isolated shell.

### 3. Bridging Agentic Workflow Gaps (Deep Research)

Based on research into how advanced AI agents operate iteratively, here are three "gaps" we should strongly consider bridging into `devx shell` to make the workspace truly agent-native:

- **A. Global Skills Vault (`~/.agent/skills`):** Many developers maintain a global folder of AI system prompts, "skills", or standard operating procedures on their Mac. We should automatically mount `~/.agent` or `~/.gemini/antigravity/` into the container so agents running inside don't lose access to the developer's generalized skill sets.
- **B. Agentic Sandboxing (Podman/Docker Socket):** Advanced agents (like Devin or SWE-Agent) often want to securely run `docker build` or execute unsafe code in an ephemeral container. If we run the agent *inside* `devx shell`, it has no Docker daemon. We should mount the `podman.sock` into the dev container as `/var/run/docker.sock` so the AI agent has Docker-in-Docker (DooD) privileges to spin up its own tests.
- **C. Seamless Git/SSH Agent Forwarding:** Instead of manually configuring Git credentials inside the devcontainer, `devx shell` should ensure the host's `SSH_AUTH_SOCK` and Git credential helper configurations are perfectly forwarded. If Claude Code decides to commit and `git push`, it shouldn't fail with a 403 or hang on an SSH passphrase prompt.

## Implementation Steps

1. **`internal/ai/bridge.go`:** Create the TCP dialer for Ollama/LM Studio discovery and define the env var overrides.
2. **`internal/devcontainer/mounts.go`:** Add the logic to safely probe the user's home directory (`os.UserHomeDir()`) for the Agent config paths (Claude, Opencode, Gemini, Codex) and construct the `-v` bind mount arrays.
3. **`cmd/shell.go`:** Integrate the bridge logging/discovery output.
4. Update `IDEAS.md` (remove #28) and update `docs/guide/ai-agents.md` documenting this new capability.
