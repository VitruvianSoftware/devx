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
