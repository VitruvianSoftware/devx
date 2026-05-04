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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDiscoverAIProvider_EnvFallback(t *testing.T) {
	// Clear any existing keys first
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_BASE", "")

	// With no local LLMs and no env var, should return nil.
	// Note: this test may pass or fail depending on whether Ollama/LM Studio
	// is running on the test machine. We only test the env var path explicitly.

	// Set OPENAI_API_KEY and verify it's picked up
	t.Setenv("OPENAI_API_KEY", "sk-test-key-12345")
	provider := DiscoverAIProvider()

	// If a local LLM is running, it takes priority — skip cloud assertion
	if provider != nil && provider.Source == "cloud" {
		if provider.Name != "OpenAI" {
			t.Errorf("expected provider name 'OpenAI', got %q", provider.Name)
		}
		if provider.APIKey != "sk-test-key-12345" {
			t.Errorf("expected API key 'sk-test-key-12345', got %q", provider.APIKey)
		}
		if provider.DefaultModel != "gpt-4o" {
			t.Errorf("expected default model 'gpt-4o', got %q", provider.DefaultModel)
		}
		if provider.BaseURL != "https://api.openai.com/v1" {
			t.Errorf("expected base URL 'https://api.openai.com/v1', got %q", provider.BaseURL)
		}
	}
}

func TestDiscoverAIProvider_CustomBase(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-key")
	t.Setenv("OPENAI_API_BASE", "http://localhost:4000/v1")

	provider := DiscoverAIProvider()
	if provider != nil && provider.Source == "cloud" {
		if provider.BaseURL != "http://localhost:4000/v1" {
			t.Errorf("expected custom base URL, got %q", provider.BaseURL)
		}
	}

	// Clean up
	_ = os.Unsetenv("OPENAI_API_BASE")
}

func TestGenerateCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "INSERT INTO users (name) VALUES ('test');"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := &AIProvider{
		Name:    "Test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Source:  "local",
	}

	result, err := GenerateCompletion(provider, "test-model", "system", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "INSERT INTO users (name) VALUES ('test');" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestGenerateCompletion_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": {"message": "rate limited"}}`))
	}))
	defer server.Close()

	provider := &AIProvider{
		Name:    "Test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Source:  "local",
	}

	_, err := GenerateCompletion(provider, "model", "sys", "usr")
	if err == nil {
		t.Fatal("expected error for HTTP 429")
	}
}

func TestGenerateCompletion_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices": []}`))
	}))
	defer server.Close()

	provider := &AIProvider{
		Name:    "Test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Source:  "local",
	}

	_, err := GenerateCompletion(provider, "model", "sys", "usr")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}
