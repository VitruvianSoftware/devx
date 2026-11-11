# Updater

## Overview
Component package providing implementation for `apps/cli/devx/internal/updater`.

## Interface
- `[updater.go:CheckResult]`
- `[updater.go:Check]`
- `[updater.go:CheckForUpgrade]`
- `[upgrade.go:DownloadURL]`
- `[upgrade.go:ChecksumURL]`
- `[upgrade.go:SelfUpgrade]`

## Invariants
- None

## Side Effects
- [updater.go:29]: `"net/http"`
- [updater.go:76]: `// Fetch fresh from GitHub`
- [updater.go:125]: `client := &http.Client{Timeout: timeout}`
- [upgrade.go:30]: `"net/http"`
- [upgrade.go:108]: `defer func() { _ = os.Remove(tmpPath) }()`
- [upgrade.go:137]: `return os.Remove(f.Name())`

## Verification
- Run: `bazel test //apps/cli/devx/internal/updater/...`
