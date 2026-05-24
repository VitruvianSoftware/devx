# Design: Opt-in unified log streaming for `devx up`

**Date:** 2026-05-24
**Status:** Approved (design); pending implementation plan
**Topic:** Bring skaffold-style container log streaming to `devx up`, across all runtimes, behind an opt-in parameter.

**Scope note (updated during planning):** Implementing this across *all* runtimes
surfaced that `runtime: container` has no deployer or config schema in `devx up`
today — the `RuntimeContainer` enum exists ([dag.go:49](../../../internal/orchestrator/dag.go))
but `Execute` has no dispatch case ([dag.go:337-347](../../../internal/orchestrator/dag.go)),
so a container service silently falls through to the host path. Per decision, this
effort now *also* implements the `runtime: container` deployer as a prerequisite,
with a new `container:` config block supporting both a pre-built `image:` and a
`build:` block. `runtime: bridge` remains out of scope (it produces no workload
logs). The work is split into two independently-shippable plans: **(1) the
container runtime**, **(2) unified log streaming**.

## Motivation

`skaffold dev`/`debug` stream deployed container logs back to the developer's
terminal, color-coded and prefixed per workload. `devx up` is a long-running
foreground command (it blocks until Ctrl+C while holding port-forwards/tunnels
open, [cmd/up.go:452](../../../cmd/up.go)), so that window is the natural place
to show the same thing — today it just idles on a signal.

devx already does exactly this for `runtime: host` services: `startHostProcess`
tees the child process output to both the terminal and a log file
([internal/orchestrator/dag.go:461](../../../internal/orchestrator/dag.go)):

```go
cmd.Stdout = io.MultiWriter(os.Stdout, logFile) // logFile = ~/.devx/logs/<name>.log
```

So host logs stream inline in `up` **and** land in a file that `devx logs`
independently tails ([internal/logs/streamer.go:149](../../../internal/logs/streamer.go)
`watchHostLogs` → `tailFile`). The **file is the cross-process fan-out point**.
The other runtimes (`container`, `kubernetes`, `cloud`) don't participate
because their workload isn't a child process of `up`.

This design generalizes the host pattern to every runtime.

## How skaffold does it (reference)

Verified against skaffold v2.21.0 (`pkg/skaffold/kubernetes/`):

- A **PodWatcher** (client-go) watches pods matching a selector and emits
  add/modify/delete events.
- A **LogAggregator** subscribes; for each container (init + regular) with a
  `ContainerID` not already tracked, it spawns a log stream. Dedup is keyed on
  `ContainerID`, so a restart (new ID) is automatically re-tailed.
- The stream itself **shells out**: `kubectl logs --since=Xs -f <pod> -c <ctr>
  --namespace <ns>`, piped line-by-line through a formatter. `--since=Xs`
  (rounded up) is used instead of `--since-time` to avoid laptop/server clock
  skew and to avoid re-dumping history on reconnect.
- A **formatter** prefixes each line with `[pod container]` and colorizes via a
  **ColorPicker** that assigns a stable color per image. Coloring is gated on a
  TTY check.

The log-byte path is a plain `kubectl logs` subprocess — the same pattern devx
already uses everywhere — so it ports directly.

## Goals

- `devx up`, when opted in, streams logs from all runtime workloads it started,
  inline in the `up` terminal, skaffold-style (per-service prefix + color).
- The same logs are simultaneously available to a separately-running `devx logs`
  TUI, with no coordination between the two processes.
- Off by default for the newly covered runtimes; existing `up` output is
  unchanged unless opted in.

## Non-goals

- Standalone cluster log tailing from `devx logs` with no `up` running (the
  file fan-out means `up` is the producer). Could be added later via independent
  double-tailing; out of scope here.
- Log search/persistence/shipping beyond the existing `~/.devx/logs/` files.
- `runtime: bridge` log production — bridge is connectivity (intercept /
  port-forward), not a workload, so it produces no logs of its own.

## Confirmed decisions

- **A — k8s pod discovery: robust (skaffold parity).** Watch pods via
  `kubectl get pods` with watch events; per new containerID, tail with
  `kubectl logs --since -f`; dedupe by containerID (restart ⇒ re-tail); include
  init containers; surface waiting-state messages (e.g. `ImagePullBackOff`).
  Stays client-go-free.
- **B — `devx logs` single source of truth.** `~/.devx/logs/*.log` is the one
  source for anything `up` produces. `devx logs` dedupes by service name
  (preferring the file source) so container services don't appear twice once
  `up` also writes their logs to files. File-sourced lines are relabeled from
  `host:<name>` ([internal/logs/streamer.go:165](../../../internal/logs/streamer.go))
  to the plain service name plus a runtime tag, so the TUI colors/labels them
  correctly.
- **C — opt-in precedence & host default.** Resolution order:
  `--logs`/`--no-logs` flag > per-service `logs:` > top-level `logs:` >
  built-in default. Built-in defaults: **host = on** (preserves today's
  behavior), **container/kubernetes/cloud = off**. Flag/config override
  uniformly, including muting host. So `devx up` is byte-for-byte today's
  behavior; `devx up --logs` lights up every runtime. Because today's host logs
  are **raw** (unprefixed), the default host path keeps the existing
  `MultiWriter(os.Stdout, file)` formatting; the shared prefixing/coloring
  `LineWriter` is applied only when inline streaming is explicitly enabled (flag
  or `logs:` config) — the multi-service case where `[service]` prefixes are what
  disambiguate interleaved output. This is what keeps plain `devx up`
  byte-identical.
- **D — implement `runtime: container` (prerequisite).** `devx up` has no
  container deployer today, so there's no container workload to stream. This
  effort adds a real `startContainerNode` (build-or-run image, port-map via
  `network.ResolvePort`, env, sync bind-mounts, `devx-svc-<name>` naming +
  `managed-by=devx` labels, `rm -f` cleanup) and a `RuntimeContainer` dispatch
  case, so container services actually run in `up` and inherit the generic
  HTTP/TCP healthcheck like host.
- **E — container image model: both `image:` and `build:`.** A new per-service
  `container:` block accepts a pre-built `image:` OR a `build:` block
  (context/dockerfile/tag/platforms, reusing the `internal/image` builder +
  `image.Spec`), mutually exclusive. Compose-style.

## Architecture

One producer per service, two sinks, mirroring the host tee:

```
log source ──▶ io.MultiWriter(
                  inlineWriter,                 // [service] prefixed + colored + redacted → up's stdout
                  ~/.devx/logs/<service>.log,   // durable fan-out: devx logs tails this
               )
```

- **Inline writer** = the in-`up` skaffold-style view (prefix + color). Attached
  when inline streaming is enabled for that service (Decision C). At the built-in
  default (host only), host keeps its current **raw** inline tee; the prefixing
  writer is used once inline streaming is explicitly opted in.
- **File sink** = the cross-process fan-out. Always written when streaming is
  enabled for the service; consumed by `devx logs` unchanged.
- Per-runtime, the only difference is **how the log source is obtained**; the
  tee, prefixing, coloring, redaction, and lifecycle are shared.

### Components

| Component | Location | Responsibility |
|---|---|---|
| `LineWriter` | `internal/logs/prefix.go` *(new)* | Wrap an `io.Writer`; on each complete line, prefix `[service]` in the service color, redact secrets, write. Line-buffered. |
| Shared `ColorPicker` | `internal/logs/` *(extract from [tui.go:35](../../../internal/logs/tui.go))* | Stable service→color assignment, shared by the inline view and the TUI so colors agree. |
| `OpenServiceTee(service, opts)` | `internal/logs/` *(new)* | Build `io.MultiWriter(inlineWriter?, logFile)`; create/truncate `~/.devx/logs/<service>.log`; return writer + closer. |
| k8s log producer | `internal/orchestrator/kubernetes_logs.go` *(new)* | Pod watch + per-container `kubectl logs -f` → tee. Skaffold semantics (below). |
| cloud log producer | `internal/orchestrator/cloudrun_logs.go` *(new)* | `gcloud` log tail for the deployed service → tee. |
| container producer | new `internal/orchestrator/container_node.go` | `<rt> logs --tail N -f devx-svc-<name>` → tee (reuses the tail shape from [streamer.go:99](../../../internal/logs/streamer.go)). Depends on the new container deployer (Decision D). |
| host producer | adapt [dag.go:461](../../../internal/orchestrator/dag.go) | Keep today's raw `MultiWriter(os.Stdout, file)` at default; route through the `LineWriter` + file tee only when inline streaming is opted in. |
| config + flag | [cmd/devxconfig.go](../../../cmd/devxconfig.go), [cmd/up.go](../../../cmd/up.go) | `Logs *bool` field (top-level default + per-service) and `--logs`/`--no-logs`. |
| `devx logs` dedup/relabel | [cmd/logs.go](../../../cmd/logs.go), [streamer.go](../../../internal/logs/streamer.go) | Single-source dedupe by service; relabel file lines. |

### k8s producer detail (Decision A)

After `startKubernetesNode` deploys and `kubectl wait ... Available` returns:

1. **Watch:** stream pod events with
   `kubectl get pods -n <ns> -w --output-watch-events -o json` (optionally
   narrowed with `-l <selector>` to the label selectors of the workloads devx
   just applied, to avoid over-capturing unrelated pods in the namespace).
   Decode the `{type, object}` event stream incrementally.
2. **Tail:** for each pod event, for each container in
   `InitContainerStatuses + ContainerStatuses` whose `ContainerID` is set and not
   yet tracked, spawn `kubectl logs --since=<Xs> -f <pod> -c <ctr> -n <ns>` and
   pipe it through the service tee. Track by `ContainerID`.
3. **Restarts / new replicas:** a restart yields a new `ContainerID`, so it is
   re-tailed automatically; new pods arrive as new watch events.
4. **Waiting state:** if a container is `Waiting` with a message
   (`ImagePullBackOff`, `CrashLoopBackOff`, …), print that message once so the
   developer sees *why* there are no logs.
5. **`--since`:** computed as `ceil(time.Since(start))` seconds, mirroring
   skaffold, to avoid clock skew and history re-dumps on reconnect.

### Cloud Run producer detail

`runtime: cloud` logs live in Cloud Logging, not on a tail-able pod. Stream them
through gcloud's log-tailing surface for the deployed service (exact subcommand —
`gcloud beta run services logs tail` vs. `gcloud logging tail` with a Cloud Run
resource filter — confirmed against the installed gcloud during implementation,
since this surface is in beta). Output is teed identically. This is the last
phase and the least certain mechanism; if gcloud streaming proves unreliable,
fall back to periodic `gcloud ... logs read` polling.

### Opt-in resolution (Decision C)

```
effective(service) =
    flag (--logs / --no-logs)            if set      // applies to all services
    else per-service `logs:` in devx.yaml if set
    else top-level `logs:` in devx.yaml   if set
    else built-in default                             // host=on, others=off
```

`--logs` and `--no-logs` are mutually exclusive booleans on `upCmd`. The
config field is `*bool` at both top level and per service so "unset" is
distinguishable from "false."

### Lifecycle

All producers are children of the existing `up` context
([cmd/up.go:400](../../../cmd/up.go)) and are torn down by `dagCleanup` / context
cancellation on Ctrl+C. Streaming simply fills the existing
block-until-signal window ([cmd/up.go:452](../../../cmd/up.go)) — no new blocking
model is introduced. The cloudflared-tunnel branch already blocks on a foreground
process; in that branch, inline streaming runs concurrently and is cancelled the
same way.

### `devx logs` reconciliation (Decision B)

- The streamer keeps `watchHostLogs`/`tailFile` as the single path for
  everything `up` produces (host + container + k8s + cloud all write files).
- `watchContainers` (live container discovery) is retained but deduped against
  the file source by service name, so a container that `up` is already teeing to
  a file is not also tailed directly (no double display).
- `LogLine.Type` gains values beyond `container`/`host` (e.g. `kubernetes`,
  `cloud`); file discovery derives the service name without the `host:` prefix
  and sets the correct type for coloring/labeling.

## Error handling

- **Pod not ready / image pull failures:** surfaced via the waiting-state
  message path (above), not silent.
- **Stream drop while workload lives:** reconnect the `kubectl logs -f` (or
  container/gcloud) subprocess with a fresh `--since` to resume without
  duplicating already-shown lines.
- **Missing CLIs:** `kubectl`/`gcloud` availability is already validated by the
  deploy path before streaming starts; no new preflight needed.
- **File growth:** `~/.devx/logs/<service>.log` is truncated at the start of an
  `up` run (`O_CREATE|O_WRONLY|O_TRUNC`). This intentionally changes host logs
  from append to truncate-per-run, giving fresh per-run logs and bounding size;
  `tail -n 50 -f` consumers are unaffected.
- **Redaction:** the `LineWriter` redacts on the inline path too (today only the
  streamer redacts), so secrets never reach the terminal or the file.

## Testing

- **Unit:** `LineWriter` (prefix, color on/off by TTY, redaction, partial-line
  buffering across writes); the `kubectl get pods --output-watch-events` JSON
  event decoder; containerID dedup and restart re-tail; opt-in precedence
  resolution (flag > per-service > top-level > default, including host default).
- **Integration:** k8s producer against the configured kube context (or a fake
  `kubectl` on PATH emitting canned watch-event + logs output); verify a service
  appears both inline and in `~/.devx/logs/<service>.log`, and that a
  concurrently-run `devx logs` shows it exactly once.
- **Regression:** `devx up` with no flag produces byte-identical output to today
  for a host-only project.

## Phasing

Split into two independently-shippable plans:

**Plan 1 — `runtime: container` deployer** (prerequisite; standalone value: makes
container services actually run in `up`):
1. `container:` config schema (`image` | `build`) + parsing.
2. `ContainerNodeConfig` + up.go mapping.
3. `startContainerNode` (image path) + `RuntimeContainer` dispatch case.
4. `startContainerNode` (build path via `internal/image`).
5. Cleanup (`rm -f`) registration in the DAG.

**Plan 2 — unified log streaming** (host + container + k8s + cloud):
1. `ColorPicker` extraction (thread-safe) + TUI refactor.
2. `LineWriter` (prefix + color + redact, line-buffered).
3. `OpenServiceTee` + `--logs`/`--no-logs` flag + `logs:` config + precedence resolver.
4. Host producer onto the tee (raw by default; prefixed when opted in).
5. Container producer (tail `devx-svc-<name>`) — depends on Plan 1.
6. k8s producer (pod watch + per-container tail + dedup + waiting messages).
7. Cloud Run producer (`gcloud` tail).
8. `devx logs` single-source dedupe + relabel.

The opt-in flag/config lands early in Plan 2 (step 3) so every subsequent producer
is gated correctly. Plan 2's container producer (step 5) depends on Plan 1.
