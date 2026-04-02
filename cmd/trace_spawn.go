package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/telemetry"
	"github.com/spf13/cobra"
)

var traceSpawnRuntime string
var traceSpawnPersist bool

var traceSpawnCmd = &cobra.Command{
	Use:   "spawn [engine]",
	Short: "Spawn a local distributed tracing backend",
	Long: `Spawn a local OpenTelemetry-compatible tracing backend.

Engines:
  jaeger   (default) — Lightweight Jaeger All-in-One. UI: http://localhost:16686
  grafana            — Full Grafana LGTM stack (OTel Collector + Tempo + Loki + Prometheus + Grafana). UI: http://localhost:3000

'devx shell' automatically detects a running backend and injects
OTEL_EXPORTER_OTLP_ENDPOINT into your dev container, so any standard OTEL SDK
beams traces locally with zero .env changes required.

Use --persist to retain trace and metric data across restarts.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTraceSpawn,
}

func init() {
	traceSpawnCmd.Flags().StringVar(&traceSpawnRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	traceSpawnCmd.Flags().BoolVar(&traceSpawnPersist, "persist", false,
		"Persist trace data to ~/.devx/telemetry/<engine>/ across restarts")
	traceCmd.AddCommand(traceSpawnCmd)
}

func runTraceSpawn(_ *cobra.Command, args []string) error {
	engineName := telemetry.EngineJaeger // default
	if len(args) == 1 {
		engineName = telemetry.Engine(strings.ToLower(args[0]))
	}

	runtime := traceSpawnRuntime
	containerName := telemetry.ContainerName(engineName)

	// Check if already running
	checkOut, err := exec.Command(runtime, "inspect", containerName, "--format", "{{.State.Running}}").Output()
	if err == nil && strings.TrimSpace(string(checkOut)) == "true" {
		fmt.Printf("✓ %s backend is already running.\n", engineName)
		printTraceInfo(engineName)
		return nil
	}

	// Remove any stopped container with the same name
	_ = exec.Command(runtime, "rm", "-f", containerName).Run()

	// Resolve persist directory if requested
	persistDir := ""
	if traceSpawnPersist {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory: %w", err)
		}
		persistDir = filepath.Join(home, ".devx", "telemetry", string(engineName))
		if err := os.MkdirAll(persistDir, 0755); err != nil {
			return fmt.Errorf("creating persist directory %s: %w", persistDir, err)
		}
		fmt.Printf("💾 Persisting trace data to %s\n", persistDir)
	}

	runArgs, cfg, err := telemetry.SpawnArgs(engineName, runtime, persistDir)
	if err != nil {
		return err
	}

	fmt.Printf("🔍 Spawning %s on OTLP ports 4317/4318...\n", cfg.Name)

	cmd := exec.Command(runtime, runArgs...)
	var stderrBuf bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderrBuf.String(), "address already in use") ||
			strings.Contains(stderrBuf.String(), "port is already allocated") {
			return fmt.Errorf("a required port (4317, 4318, or %d) is already in use", cfg.UIPort)
		}
		return fmt.Errorf("failed to start %s: %w", cfg.Name, err)
	}

	fmt.Println()
	fmt.Printf("✅ %s is running!\n\n", cfg.Name)
	printTraceInfo(engineName)
	return nil
}

func printTraceInfo(engine telemetry.Engine) {
	switch engine {
	case telemetry.EngineJaeger:
		fmt.Println("  OTLP gRPC:  :4317")
		fmt.Println("  OTLP HTTP:  :4318")
		fmt.Println("  Jaeger UI:  http://localhost:16686")
	case telemetry.EngineGrafana:
		fmt.Println("  OTLP gRPC:  :4317")
		fmt.Println("  OTLP HTTP:  :4318")
		fmt.Println("  Grafana UI: http://localhost:3000  (admin / admin)")
		fmt.Println("  Prometheus: http://localhost:9090")
		fmt.Println("  Tempo:      :3200")
	}
	fmt.Println()
	fmt.Println("  Run 'devx shell' to have OTEL_EXPORTER_OTLP_ENDPOINT auto-injected.")
	fmt.Printf("  Stop: devx trace rm %s\n", engine)
}
