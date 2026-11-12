# Mcpinstall

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/mcpinstall`.

## Interface
- `[host.go:Scope]`
- `[host.go:String]`
- `[host.go:Status]`
- `[host.go:InstallResult]`
- `[host.go:Options]`
- `[host.go:Host]`
- `[host_codex.go:Key]`
- `[host_codex.go:DisplayName]`
- `[host_codex.go:Detect]`
- `[host_codex.go:Status]`
- `[host_codex.go:Install]`
- `[host_codex.go:Uninstall]`
- `[host_json.go:ServerEntryKey]`
- `[host_json.go:Key]`
- `[host_json.go:DisplayName]`
- `[host_json.go:Detect]`
- `[host_json.go:Status]`
- `[host_json.go:Install]`
- `[host_json.go:Uninstall]`
- `[host_manual.go:Key]`

## Invariants
- None

## Side Effects
- [host_codex.go:27]: `"os/exec"`
- [host_codex.go:42]: `if _, err := exec.LookPath("codex"); err == nil {`
- [host_codex.go:202]: `if err := os.WriteFile(tmp, data, 0o644); err != nil {`
- [host_json.go:28]: `"os/exec"`
- [host_json.go:45]: `// detectBins are checked via exec.LookPath. Finding any one is a hit.`
- [host_json.go:69]: `if _, err := exec.LookPath(bin); err == nil {`
- [host_manual.go:27]: `"os/exec"`
- [host_manual.go:50]: `if _, err := exec.LookPath(bin); err == nil {`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/mcpinstall/...`
