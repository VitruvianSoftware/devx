# Ai

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/ai`.

## Interface
- `[agent.go:AgentMode]`
- `[agent.go:AgentResult]`
- `[agent.go:RunAgentPrompt]`
- `[bridge.go:BridgeEnv]`
- `[bridge.go:DiscoverHostLLMs]`
- `[completion.go:AIProvider]`
- `[completion.go:DiscoverAIProvider]`
- `[completion.go:GenerateCompletion]`
- `[diagnose.go:DiagnoseFailure]`

## Invariants
- [diagnose.go:92]: `Diagnosis:  "Ollama is not running. AI features require the Ollama inference ...`

## Side Effects
- [agent.go:25]: `"os/exec"`
- [agent.go:75]: `ollamaPath, err := exec.LookPath("ollama")`
- [agent.go:81]: `listOut, err := exec.Command(ollamaPath, "list").Output()`
- [bridge.go:52]: `"OPENAI_API_BASE":    fmt.Sprintf("http://%s:11434/v1", hostHostname),`
- [bridge.go:54]: `"ANTHROPIC_BASE_URL": fmt.Sprintf("http://%s:11434", hostHostname),`
- [bridge.go:55]: `"GEMINI_API_BASE":    fmt.Sprintf("http://%s:11434", hostHostname),`
- [completion.go:29]: `"net/http"`
- [completion.go:37]: `BaseURL      string // OpenAI-compatible API base (e.g., "http://127.0.0.1:11...`
- [completion.go:54]: `BaseURL:      "http://127.0.0.1:11434/v1",`
- [diagnose.go:26]: `"os/exec"`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/ai/...`
