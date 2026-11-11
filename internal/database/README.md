# Database

## Overview
Component package providing implementation for `apps/cli/devx/internal/database`.

## Interface
- `[engine.go:Engine]`
- `[engine.go:Registry]`
- `[engine.go:ConnString]`
- `[engine.go:SupportedEngines]`
- `[query.go:CannedQuery]`
- `[query.go:CannedQueries]`
- `[query.go:QueryResult]`
- `[query.go:ExecuteReadOnlyQuery]`
- `[query.go:ExecuteQuery]`
- `[query.go:RenderTable]`
- `[snapshot.go:SnapshotMeta]`
- `[snapshot.go:SnapshotDir]`
- `[snapshot.go:CreateSnapshot]`
- `[snapshot.go:RestoreSnapshot]`
- `[snapshot.go:ListSnapshots]`
- `[snapshot.go:DeleteSnapshot]`
- `[synthesizer.go:SynthesizableEngines]`
- `[synthesizer.go:IsSynthesizable]`
- `[synthesizer.go:ExtractSchema]`
- `[synthesizer.go:SanitizeLLMSQL]`

## Invariants
- None

## Side Effects
- [engine.go:22]: `// configurations for one-click local provisioning via devx db spawn.`
- [query.go:25]: `"os/exec"`
- [query.go:33]: `// CannedQuery defines a pre-built diagnostic query.`
- [query.go:35]: `Name        string            // Short name for the query (e.g., "sizes")`
- [snapshot.go:28]: `"os/exec"`
- [snapshot.go:84]: `var cmd *exec.Cmd`
- [snapshot.go:99]: `_ = os.Remove(tarPath)`
- [synthesizer.go:25]: `"os/exec"`
- [synthesizer.go:51]: `var cmd *exec.Cmd`
- [synthesizer.go:54]: `cmd = exec.Command(runtime, "exec", "-i", containerName,`

## Verification
- Run: `bazel test //apps/cli/devx/internal/database/...`
