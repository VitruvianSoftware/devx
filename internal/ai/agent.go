// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ai

import (
	"fmt"
	"os/exec"
	"strings"
)

// AgentMode describes how an AI prompt was executed.
type AgentMode string

const (
	AgentModeOllamaLaunch AgentMode = "ollama_launch" // Full agentic via ollama launch
	AgentModeChatAPI      AgentMode = "chat_api"       // Simple chat completion via internal/ai
	AgentModeNone         AgentMode = "none"            // No AI available
)

// AgentResult holds the output of an AI-assisted operation.
type AgentResult struct {
	Output string    // The AI-generated text
	Mode   AgentMode // Which execution path was used
}

// ollamaLaunchAgents maps agent names to their ollama launch integration name.
var ollamaLaunchAgents = []string{"claude", "opencode", "codex"}

// RunAgentPrompt executes a prompt using the best available AI backend.
//
// Priority:
//  1. ollama launch <agent> -- -p "prompt" --permission-mode plan --print
//     (full agentic, can read files and understand codebase context)
//  2. internal/ai chat completion via DiscoverAIProvider
//     (simple prompt/response, no file access)
//  3. Returns AgentModeNone if no AI is available
//
// The prompt should be self-contained — the ollama launch path has file access
// but the chat API fallback does not, so include any necessary context in the prompt.
func RunAgentPrompt(prompt string) (*AgentResult, error) {
	// Try ollama launch first (agentic, file-aware)
	if result, err := tryOllamaLaunch(prompt); err == nil {
		return result, nil
	}

	// Fallback to chat completion API
	if result, err := tryChatCompletion(prompt); err == nil {
		return result, nil
	}

	// No AI available
	return &AgentResult{Mode: AgentModeNone}, nil
}

// tryOllamaLaunch attempts to run a prompt via ollama launch with a supported agent.
func tryOllamaLaunch(prompt string) (*AgentResult, error) {
	ollamaPath, err := exec.LookPath("ollama")
	if err != nil {
		return nil, fmt.Errorf("ollama not found")
	}

	// Get first available model to avoid interactive prompt
	listOut, err := exec.Command(ollamaPath, "list").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list ollama models")
	}
	
	lines := strings.Split(strings.TrimSpace(string(listOut)), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("no ollama models found (run ollama pull first)")
	}
	
	fields := strings.Fields(lines[1])
	if len(fields) == 0 {
		return nil, fmt.Errorf("invalid ollama list output")
	}
	model := fields[0]

	// Find a supported agent that's installed
	for _, agent := range ollamaLaunchAgents {
		if _, err := exec.LookPath(agent); err != nil {
			continue
		}

		// Run: ollama launch <agent> --model <model> -- -p "prompt" --permission-mode plan
		// plan mode = read-only (safe), -p = print mode (non-interactive)
		cmd := exec.Command(ollamaPath, "launch", agent, "--model", model, "--",
			"-p", prompt,
			"--permission-mode", "plan",
		)

		out, err := cmd.Output()
		if err != nil {
			continue // Try next agent
		}

		output := strings.TrimSpace(string(out))
		if output == "" {
			continue
		}

		return &AgentResult{
			Output: output,
			Mode:   AgentModeOllamaLaunch,
		}, nil
	}

	return nil, fmt.Errorf("no ollama launch agents available")
}

// tryChatCompletion attempts to run a prompt via the internal chat completion API.
func tryChatCompletion(prompt string) (*AgentResult, error) {
	provider := DiscoverAIProvider()
	if provider == nil {
		return nil, fmt.Errorf("no AI provider found")
	}

	systemPrompt := "You are a helpful assistant integrated into a CLI developer tool called devx. Be concise and direct. Output only what is asked for — no explanations, no markdown formatting, no code fences."

	output, err := GenerateCompletion(provider, "", systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	return &AgentResult{
		Output: strings.TrimSpace(output),
		Mode:   AgentModeChatAPI,
	}, nil
}
