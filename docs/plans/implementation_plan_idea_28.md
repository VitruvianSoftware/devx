# Idea 28: Zero-Friction Local AI Inference

This plan covers the implementation of `devx ai spawn` to provide local AI inference capabilities via Ollama, reducing reliance on expensive OpenAI API calls during local dev/test loops.

## User Review Required

> [!IMPORTANT]
> **GPU Acceleration in Containers:** Running Ollama in a Podman/Docker container under macOS (Apple Silicon) generally restricts it to CPU-only execution unless specific virtualization paths (like OrbStack's metal acceleration) are used. Native Linux users with NVIDIA GPUs can use `--gpus all`. 
> Do we want to automatically detect the OS and runtime (`orbstack` vs `docker` vs `podman`) to inject the optimal GPU flags (e.g. `--gpus all` or `--device /dev/dri`), or should we just keep it simple and CPU-bound by default with an optional `--gpu` flag?

> [!NOTE]
> **Model Fetching:** `devx ai spawn` will start the inference engine on port 11434. Should the command also support a `--model <name>` flag (e.g., `--model llama3.2`) that automatically pulls the model upon starting, or should we leave it to the user to hit the API/run commands inside the container after it's started?

## Proposed Changes

---

### `cmd/` (CLI Commands)

#### [NEW] `cmd/ai.go`
- Registers the root `devx ai` command category.

#### [NEW] `cmd/ai_spawn.go`
- Implements `devx ai spawn [engine]`.
- Will default to `ollama` as the engine.
- Starts `docker.io/ollama/ollama:latest` with the label `devx-ai=ollama` and `managed-by=devx`.
- Binds port `11434` by default.
- Printout will emulate how cloud/mail emulators work, showing injected variables.

#### [NEW] `cmd/ai_list.go` & `cmd/ai_rm.go`
- List and remove spawned AI container(s), consistent with the `devx cloud` and `devx mail` UX patterns.

---

### `internal/ai/` (Core Logic)

#### [NEW] `internal/ai/inference.go`
- Handles the actual container execution args.
- Logic for exporting the environment mapping:
  - `OPENAI_API_BASE=http://localhost:11434/v1`
  - `OPENAI_API_KEY=devx-local-ai` (dummy key for strict SDKs)
  - `OLLAMA_HOST=http://localhost:11434`

#### [NEW] `internal/ai/discovery.go`
- `DiscoverAIEnvVars(runtime string) map[string]string`
- Queries running containers for `label=devx-ai`, extracting the host port to cleanly inject the URLs into `devx shell`.

---

### `cmd/shell.go` (Shell Injection)

#### [MODIFY] `cmd/shell.go`
- Call `ai.DiscoverAIEnvVars(runtime)` before launching the devcontainer.
- Inject the returned variables (like `OPENAI_API_BASE`) directly into the shell environment natively, so any OpenAI SDK scripts automatically route to the local Ollama instance.

## Open Questions

1. Do we want to expose Ollama's storage volume `/root/.ollama` to a named volume (e.g., `devx-ai-ollama-data`) so downloaded models persist across `devx ai rm` and `devx ai spawn` cycles? (Highly recommended to avoid re-downloading multi-GB models).
2. Are there any other specific environment variables (like `ANTHROPIC_BASE_URL` if some compatibility layer is ever added) we should preemptively inject?

## Verification Plan

### Automated Tests
- Unit tests in `internal/ai` to verify `buildOllamaArgs` produces the correct paths, volume mounts, and port bindings.

### Manual Verification
- Run `devx ai spawn`.
- Run `devx shell` inside a dummy project and execute `echo $OPENAI_API_BASE` -> verify it points to `http://localhost:11434/v1`.
- Execute a cURL request to testing the mock OpenAI completion endpoint on the locally spun up Ollama instance to ensure it replies successfully.
