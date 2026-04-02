# Task Tracker - Idea #28: Zero-Friction Local AI Bridge

- `[x]` **1. Transparent Host API Bridging**
  - `[x]` Implement `internal/ai/bridge.go` for TCP discovery (11434, 1234).
  - `[x]` Auto-inject LLM overrides (`OPENAI_API_BASE`, `OLLAMA_HOST`, etc.).
  - `[x]` Respect existing `.env` overrides (skip injection if manually set).
  - `[x]` Add loud but clean terminal warning if bridge intercepts API traffic.

- `[x]` **2. Agent Identity Mounting**
  - `[x]` Auto-discover `~/.claude.json`, `~/.config/claude`, `~/.config/opencode`, `~/.config/gemini-cli`, `~/.codex`.
  - `[x]` Construct `-v` mount flags for `devx shell`.

- `[x]` **3. Advanced Agentic Workflow Gaps**
  - `[x]` **Global Skills:** Mount `~/.agent` and `~/.gemini/antigravity` into container.
  - `[x]` **Docker/Podman Socket (DooD):** Pass `/var/run/docker.sock` (or `/run/user/$(id -u)/podman/podman.sock`) securely into the shell.
  - `[x]` **Git/SSH Identity:** Pass `SSH_AUTH_SOCK` and mount the socket directory. Mount `~/.gitconfig` or copy credentials contextually so agents can push.

- `[x]` **4. Documentation & Cleanup**
  - `[x]` Remove Idea 28 from `IDEAS.md` and move to `FEATURES.md`.
  - `[x]` Execute the new `devx` Global Push Workflow (test -> vet -> build -> CI verification).
