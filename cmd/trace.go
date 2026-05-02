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

package cmd

import (
	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:   "trace",
	GroupID: "telemetry",
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
