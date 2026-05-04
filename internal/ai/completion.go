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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// AIProvider represents a discovered AI backend capable of chat completions.
type AIProvider struct {
	Name         string // Human-readable name (e.g., "Ollama", "LM Studio", "OpenAI")
	BaseURL      string // OpenAI-compatible API base (e.g., "http://127.0.0.1:11434/v1")
	APIKey       string // API key for authentication
	DefaultModel string // Provider-specific default model (empty = use server default)
	Source       string // "local" or "cloud"
}

// DiscoverAIProvider probes for an available AI backend using a priority cascade:
//  1. Local Ollama on port 11434
//  2. Local LM Studio on port 1234
//  3. OPENAI_API_KEY environment variable (cloud fallback)
//
// Returns nil if no AI provider is found.
func DiscoverAIProvider() *AIProvider {
	// 1. Check for Ollama (default port 11434)
	if isLocalPortOpen("11434") {
		return &AIProvider{
			Name:         "Ollama",
			BaseURL:      "http://127.0.0.1:11434/v1",
			APIKey:       "devx-local-ai",
			DefaultModel: "", // Use whatever model is loaded
			Source:       "local",
		}
	}

	// 2. Check for LM Studio (default port 1234)
	if isLocalPortOpen("1234") {
		return &AIProvider{
			Name:         "LM Studio",
			BaseURL:      "http://127.0.0.1:1234/v1",
			APIKey:       "devx-local-ai",
			DefaultModel: "", // Use whatever model is loaded
			Source:       "local",
		}
	}

	// 3. Check for OPENAI_API_KEY (cloud fallback)
	// Also supports OPENAI_API_BASE for custom endpoints (e.g., Azure, LiteLLM proxy)
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		base := "https://api.openai.com/v1"
		if customBase := os.Getenv("OPENAI_API_BASE"); customBase != "" {
			base = customBase
		}
		return &AIProvider{
			Name:         "OpenAI",
			BaseURL:      base,
			APIKey:       key,
			DefaultModel: "gpt-4o",
			Source:       "cloud",
		}
	}

	return nil
}

// chatRequest is the OpenAI-compatible chat completions request body.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the OpenAI-compatible chat completions response body.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateCompletion sends a chat completion request to an OpenAI-compatible API.
// It buffers the full response (non-streaming) and returns the completion text.
// Timeout is set to 120 seconds to accommodate large generation requests.
func GenerateCompletion(provider *AIProvider, model, systemPrompt, userPrompt string) (string, error) {
	if model == "" {
		model = provider.DefaultModel
	}
	// If still empty (local LLM with no default), omit model field and let server decide
	// by using an empty string — most OpenAI-compatible servers handle this gracefully.

	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := provider.BaseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("LLM API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// isLocalPortOpen checks if a TCP port is open on localhost.
func isLocalPortOpen(port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 100*time.Millisecond)
	if err != nil {
		return false
	}
	if conn != nil {
		_ = conn.Close()
		return true
	}
	return false
}
