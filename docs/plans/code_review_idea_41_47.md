# Code Review: `devx state` Implementation (Ideas 41 & 47)

## Files Reviewed

| File | Verdict |
|------|---------|
| [dump.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/dump.go) | 🟡 Issues found |
| [checkpoint.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/checkpoint.go) | 🟡 Issues found |
| [state.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state.go) | ✅ Clean |
| [state_dump.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_dump.go) | 🟡 Minor issue |
| [state_checkpoint.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_checkpoint.go) | ✅ Clean |
| [state_restore.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_restore.go) | ✅ Clean |
| [state_list.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_list.go) | 🟡 Minor issue |
| [state_rm.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_rm.go) | 🟡 Missing confirmation |
| [shell_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/shell_test.go) | ✅ Clean |
| [state.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/state.md) | ✅ Clean |
| [config.mjs](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/.vitepress/config.mjs) | ✅ Clean |

---

## 🔴 Bugs

### 1. Dump lists ALL containers, not just devx-managed ones
[dump.go:88-109](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/dump.go#L88-L109)

The dump runs `podman ps -a` with no filter and appends **every** container to the report. A developer running Redis, Postgres, and 3 unrelated side projects will see all of them in their diagnostic dump. The checkpoint code correctly filters to `devx-` prefixed containers, but the dump doesn't.

```diff
-	out, err := exec.Command(runtime, "ps", "-a", "--format", "{{.Names}}|{{.Image}}|{{.State}}").CombinedOutput()
+	out, err := exec.Command(runtime, "ps", "-a", "--filter", "name=devx-", "--format", "{{.Names}}|{{.Image}}|{{.State}}").CombinedOutput()
```

### 2. Orphaned checkpoint directory on partial failure
[checkpoint.go:36-79](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/checkpoint.go#L36-L79)

If `MkdirAll` succeeds on line 36 but then `getRunningDevxContainers()` returns an error or one of the goroutine checkpoints fails, the empty/partial directory is left behind. A subsequent attempt to checkpoint with the same name will fail with "already exists." The directory should be cleaned up on error.

```diff
+	// Clean up partial checkpoint on failure
+	defer func() {
+		if err != nil {
+			_ = os.RemoveAll(targetDir)
+		}
+	}()
```

### 3. `redactYamlEnv` incorrectly redacts non-env YAML keys containing colons
[dump.go:213-215](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/dump.go#L213-L215)

The `inEnv` tracker exits when it sees a line with indent `<= envIndent`, but this comparison is wrong when `env:` itself is at the root level (indent 0). Every subsequent root-level key would satisfy `indent <= 0` and correctly exit, but a blank line (indent 0, trimmed empty) would also trip the exit before `trimmed == ""` is checked. The blank-line case should be handled before the indent check.

```diff
 	for i, line := range lines {
 		trimmed := strings.TrimSpace(line)
+		if trimmed == "" {
+			continue
+		}
 		
 		// Determine current indentation
 		indent := len(line) - len(strings.TrimLeft(line, " "))
```

---

## 🟡 Design Issues

### 4. `state rm` is a destructive command with no confirmation prompt
[state_rm.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_rm.go)

`devx state rm` calls `os.RemoveAll` on the checkpoint directory with zero interactive confirmation. Every other destructive command in the codebase (`db snapshot restore`, `nuke`, `state restore`, `state checkpoint`) has an interactive `Continue? [y/N]` guard. This one skips it entirely. It should follow the established pattern.

### 5. `state list` output is sparse — missing timestamps and sizes
[state_list.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_list.go)

Compare this to `devx db snapshot list` which shows `NAME | ENGINE | SIZE | CREATED`. `devx state list` only shows the name. A checkpoint could be 2GB. Users need to know how old it is and how much disk it's consuming. The metadata (size, created time) should be gathered from the directory stat and archive file sizes.

### 6. `state dump` Tailscale status is oversimplified
[state_dump.go:40-43](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/state_dump.go#L40-L43)

The Tailscale status is `"ok"` if the VM is running, `"down"` otherwise. But the existing `cmd/status.go` actually calls `tailscale.StatusWithSSH()` which returns the real Tailscale state string. The dump should use the same function for parity — the whole *point* of a diagnostic dump is accurate state.

### 7. Parallel checkpoint/restore goroutines with shared port binding
[checkpoint.go:109-119](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/state/checkpoint.go#L109-L119)

`RestoreCheckpoint` launches goroutines to restore containers in parallel. But CRIU restore re-binds the original ports. If two containers originally bound to the same host port (e.g., via port mapping), parallel restores will race. Sequential restoration would be safer here — especially since the number of containers is typically small (3-8).

---

## 🟢 What's Good

- **Redaction logic is solid.** The env-file redactor correctly preserves comments and blank lines while replacing values. The YAML redactor correctly tracks indentation scope.
- **Consistent CLI patterns.** All commands respect `--dry-run`, `--json`, and `-y` / `--non-interactive` flags, matching the rest of the codebase.
- **Error aggregation in checkpoint/restore.** Using a buffered error channel + WaitGroup is the right pattern for collecting errors from concurrent operations.
- **Documentation is thorough.** The `state.md` guide covers usage, limitations, and the CRIU provider restriction clearly.
- **Shell test fix is correct.** Gracefully handling the port collision when a real Ollama instance is running is the right approach.

---

## Summary

The architecture is sound and the command hierarchy is well-structured, but there are **3 bugs** that should be fixed before this is production-ready — the most impactful being the unfiltered container listing in the dump (which leaks non-devx container info into diagnostic reports).
