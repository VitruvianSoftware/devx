# Telemetry

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/telemetry`.

## Interface
- `[dashboard.go:SetGrafanaURL]`
- `[dashboard.go:ProvisionDashboard]`
- `[export.go:SetOtlpEndpoint]`
- `[export.go:SetServiceVersion]`
- `[export.go:Attribute]`
- `[export.go:Attr]`
- `[export.go:ExportSpan]`
- `[export.go:BuildOTLPPayload]`
- `[metrics.go:MetricEntry]`
- `[metrics.go:RecordEvent]`
- `[metrics.go:Flush]`
- `[metrics.go:NudgeIfSlow]`
- `[metrics.go:LoadMetrics]`
- `[metrics.go:ClearMetrics]`
- `[otel.go:Engine]`
- `[otel.go:SupportedEngines]`
- `[otel.go:SpawnArgs]`
- `[otel.go:ContainerName]`
- `[otel.go:DiscoverOTEL]`

## Invariants
- None

## Side Effects
- [dashboard.go:27]: `"net/http"`
- [dashboard.go:33]: `grafanaAPI    = "http://localhost:3000/api/dashboards/db"`
- [dashboard.go:67]: `"message":   "Auto-provisioned by devx trace spawn",`
- [export.go:29]: `"net/http"`
- [export.go:35]: `otlpEndpoint   = "http://localhost:4318/v1/traces"`
- [export.go:75]: `// ExportSpan posts a single span to the local OTLP/HTTP endpoint.`
- [otel.go:25]: `"os/exec"`
- [otel.go:146]: `out, err := exec.Command(`
- [otel.go:161]: `otlpEndpoint := fmt.Sprintf("http://%s:%d", hostBridge, cfg.OTLPHTTPPort)`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/telemetry/...`
