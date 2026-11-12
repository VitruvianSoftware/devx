# Logs

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/logs`.

## Interface
- `[color.go:ColorPicker]`
- `[color.go:NewColorPicker]`
- `[color.go:Color]`
- `[crashlog.go:TailContainerCrashLogs]`
- `[crashlog.go:TailHostCrashLogs]`
- `[linewriter.go:LineWriter]`
- `[linewriter.go:NewLineWriter]`
- `[linewriter.go:Write]`
- `[linewriter.go:ColorEnabled]`
- `[redactor.go:SecretRedactor]`
- `[redactor.go:NewSecretRedactor]`
- `[redactor.go:NewSecretRedactorFromPairs]`
- `[redactor.go:Redact]`
- `[sink.go:SinkMode]`
- `[sink.go:ServiceLogPath]`
- `[sink.go:OpenServiceLog]`
- `[sink.go:BuildSink]`
- `[sink.go:BuildHostSink]`
- `[streamer.go:LogLine]`
- `[streamer.go:Streamer]`

## Invariants
- None

## Side Effects
- [crashlog.go:27]: `"os/exec"`
- [crashlog.go:52]: `out, err := exec.Command(runtime, "logs", "--tail", fmt.Sprintf("%d", lines),...`
- [streamer.go:27]: `"os/exec"`
- [streamer.go:83]: `out, err := s.Runtime.Exec("ps", "--format", "{{.Names}}")`
- [streamer.go:195]: `cmd := exec.CommandContext(ctx, "tail", "-n", "50", "-f", path)`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/logs/...`
