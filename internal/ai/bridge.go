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
	"net"
	"time"
)

// BridgeEnv wraps the auto-discovered environment variables
// and a boolean indicating if an engine was found.
type BridgeEnv struct {
	EngineName string
	Active     bool
	EnvVars    map[string]string
}

// DiscoverHostLLMs probes the native host (where devx is running) for
// common local AI engines (Ollama, LM Studio). If found, it computes
// the container-to-host injected environment variables.
func DiscoverHostLLMs(runtime string) BridgeEnv {
	hostHostname := "host.containers.internal"
	if runtime == "docker" || runtime == "orbstack" {
		hostHostname = "host.docker.internal"
	}

	// 1. Check for Ollama (default port 11434)
	if isPortOpen("11434") {
		return BridgeEnv{
			EngineName: "Ollama",
			Active:     true,
			EnvVars: map[string]string{
				"OPENAI_API_BASE":    fmt.Sprintf("http://%s:11434/v1", hostHostname),
				"OPENAI_API_KEY":     "devx-local-ai",
				"ANTHROPIC_BASE_URL": fmt.Sprintf("http://%s:11434", hostHostname),
				"GEMINI_API_BASE":    fmt.Sprintf("http://%s:11434", hostHostname),
				"GEMINI_API_KEY":     "devx-local-ai",
				"OLLAMA_HOST":        fmt.Sprintf("http://%s:11434", hostHostname),
			},
		}
	}

	// 2. Check for LM Studio (default port 1234)
	if isPortOpen("1234") {
		return BridgeEnv{
			EngineName: "LM Studio",
			Active:     true,
			EnvVars: map[string]string{
				"OPENAI_API_BASE":    fmt.Sprintf("http://%s:1234/v1", hostHostname),
				"OPENAI_API_KEY":     "devx-local-ai",
				"ANTHROPIC_BASE_URL": fmt.Sprintf("http://%s:1234", hostHostname),
				"GEMINI_API_BASE":    fmt.Sprintf("http://%s:1234", hostHostname),
				"GEMINI_API_KEY":     "devx-local-ai",
			},
		}
	}

	return BridgeEnv{Active: false}
}

func isPortOpen(port string) bool {
	timeout := 100 * time.Millisecond
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		_ = conn.Close()
		return true
	}
	return false
}
