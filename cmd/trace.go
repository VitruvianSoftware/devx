package cmd

import (
	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Manage local distributed tracing and observability backends",
	Long: `Instantly spin up a local OpenTelemetry-compatible tracing backend for
shift-left observability. 'devx shell' will automatically discover a running
backend and inject OTEL_EXPORTER_OTLP_ENDPOINT into your dev container,
so any standard OTEL SDK beams traces locally with zero config.

Available engines:
  jaeger   — Lightweight Jaeger All-in-One (default). UI on :16686.
  grafana  — Full Grafana LGTM stack (OTel Collector + Tempo + Loki + Prometheus + Grafana). UI on :3000.`,
}

func init() {
	rootCmd.AddCommand(traceCmd)
}
