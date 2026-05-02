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

package telemetry

import (
	"fmt"
	"os/exec"
	"strings"
)

// Engine represents a supported local telemetry backend.
type Engine string

const (
	EngineJaeger  Engine = "jaeger"
	EngineGrafana Engine = "grafana"
)

// SupportedEngines lists all valid engine names for user-facing help text.
var SupportedEngines = []Engine{EngineJaeger, EngineGrafana}

// engineConfig holds the container configuration for a telemetry engine.
type engineConfig struct {
	Name          string
	Image         string
	LabelValue    string
	OTLPHTTPPort  int
	OTLPGRPCPort  int
	UIPort        int
	UIPath        string
	UILabel       string
	ExtraPorts    []string // additional port bindings beyond the three core ones
	PersistTarget string   // container path to mount for --persist
}

var engineRegistry = map[Engine]engineConfig{
	EngineJaeger: {
		Name:          "Jaeger All-in-One",
		Image:         "docker.io/jaegertracing/all-in-one:latest",
		LabelValue:    "jaeger",
		OTLPGRPCPort:  4317,
		OTLPHTTPPort:  4318,
		UIPort:        16686,
		UIPath:        "/",
		UILabel:       "Jaeger UI",
		PersistTarget: "/badger",
	},
	EngineGrafana: {
		Name:          "Grafana LGTM (OTel Collector + Tempo + Loki + Prometheus + Grafana)",
		Image:         "docker.io/grafana/otel-lgtm:latest",
		LabelValue:    "grafana",
		OTLPGRPCPort:  4317,
		OTLPHTTPPort:  4318,
		UIPort:        3000,
		UIPath:        "/",
		UILabel:       "Grafana UI",
		ExtraPorts:    []string{"9090:9090", "3200:3200"}, // Prometheus, Tempo
		PersistTarget: "/data",
	},
}

// SpawnArgs constructs the container run arguments for the given engine.
// If persistDir is non-empty, the host directory is bind-mounted into the
// container to preserve data across restarts.
func SpawnArgs(engine Engine, runtime, persistDir string) ([]string, engineConfig, error) {
	cfg, ok := engineRegistry[engine]
	if !ok {
		return nil, engineConfig{}, fmt.Errorf("unknown engine %q — supported: jaeger, grafana", engine)
	}

	containerName := fmt.Sprintf("devx-telemetry-%s", cfg.LabelValue)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--label", "managed-by=devx",
		"--label", fmt.Sprintf("devx-telemetry=%s", cfg.LabelValue),
		"--restart", "unless-stopped",
		"-p", fmt.Sprintf("%d:%d", cfg.OTLPGRPCPort, cfg.OTLPGRPCPort),
		"-p", fmt.Sprintf("%d:%d", cfg.OTLPHTTPPort, cfg.OTLPHTTPPort),
		"-p", fmt.Sprintf("%d:%d", cfg.UIPort, cfg.UIPort),
	}

	for _, ep := range cfg.ExtraPorts {
		args = append(args, "-p", ep)
	}

	if persistDir != "" && cfg.PersistTarget != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", persistDir, cfg.PersistTarget))
	} else {
		// Explicit ephemeral tmpfs keeps data in memory only
		if engine == EngineJaeger {
			// Jaeger uses badger storage; point it to tmpfs when not persisting
			args = append(args, "-e", "SPAN_STORAGE_TYPE=memory")
		}
	}

	// Jaeger persistence via badger requires the storage type env var
	if persistDir != "" && engine == EngineJaeger {
		args = append(args, "-e", "SPAN_STORAGE_TYPE=badger")
		args = append(args, "-e", fmt.Sprintf("BADGER_DIRECTORY_VALUE=%s/data", cfg.PersistTarget))
		args = append(args, "-e", fmt.Sprintf("BADGER_DIRECTORY_KEY=%s/key", cfg.PersistTarget))
	}

	args = append(args, cfg.Image)
	return args, cfg, nil
}

// ContainerName returns the deterministic container name for a given engine.
func ContainerName(engine Engine) string {
	cfg, ok := engineRegistry[engine]
	if !ok {
		return fmt.Sprintf("devx-telemetry-%s", engine)
	}
	return fmt.Sprintf("devx-telemetry-%s", cfg.LabelValue)
}

// DiscoverOTEL scans running containers for any devx-managed telemetry backend.
// It returns the environment variable map to inject into devx shell, including
// OTEL_EXPORTER_OTLP_ENDPOINT and standard exporter wiring.
func DiscoverOTEL(runtime string) map[string]string {
	// Determine the correct host-to-container bridge hostname
	hostBridge := "host.containers.internal" // podman default
	if runtime == "docker" {
		hostBridge = "host.docker.internal"
	}

	out, err := exec.Command(runtime, "ps",
		"--filter", "label=managed-by=devx",
		"--filter", "label=devx-telemetry",
		"--format", "{{.Labels}}",
	).Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return nil
	}

	// Find which engine is running from label values
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, e := range SupportedEngines {
			cfg := engineRegistry[e]
			if strings.Contains(line, fmt.Sprintf("devx-telemetry=%s", cfg.LabelValue)) {
				otlpEndpoint := fmt.Sprintf("http://%s:%d", hostBridge, cfg.OTLPHTTPPort)
				result := map[string]string{
					"OTEL_EXPORTER_OTLP_ENDPOINT": otlpEndpoint,
					"OTEL_TRACES_EXPORTER":        "otlp",
				}
				// Grafana LGTM accepts metrics too; Jaeger is traces-only
				if e == EngineGrafana {
					result["OTEL_METRICS_EXPORTER"] = "otlp"
					result["OTEL_LOGS_EXPORTER"] = "otlp"
				} else {
					result["OTEL_METRICS_EXPORTER"] = "none"
				}
				return result
			}
		}
	}
	return nil
}
