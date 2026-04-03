# Future Enhancements & IDEAS

This document tracks upcoming feature ideas, requests, and architectural plans for `devx`. 

When an idea is fully implemented and shipped, it is **migrated to `FEATURES.md`** to maintain an organized historical record of capabilities.

---

## Idea Template

To propose a new feature, copy the template below and add it to the appropriate priority section. Please ensure you increment the Idea Number sequentially (the last shipped feature was 33).

```markdown
### [Idea Number]. [Feature Title]
* **Priority:** 🟢 P0 | 🟢 P1 | 🟡 P2 | 🟡 P3 | 🔴 Cut
* **Effort:** Trivial | Low | Medium | High | Very High
* **Impact:** [Short impact statement]
* **The Problem:** [Describe the workflow friction, missing capability, or developer pain point]
* **The Solution:** [Describe how `devx` will solve this seamlessly. Mention potential commands, UI (TUI/logs), flags, or architecture]
* **Key files:** [Optional: List core files or packages to be added or modified]
```

---

## 🟡 P2 — Build Later (Good Ideas, Need Refinement)

### 41. Shareable Diagnostic Dumps (DONE)
* **Priority:** 🟢 DONE
* **Effort:** Low
* **Impact:** Medium — the snapshot part is easy, but the "upload to where?" question is unsolved. Uploading to Gists adds auth friction; hosting a pastebin adds ops burden. Ship a `devx state dump` first that outputs a structured JSON/markdown report to stdout. Let sharing be a V2 concern.
* **The Problem:** "It doesn't work on my machine despite the devx environment, please help." Getting context on another developer's broken state requires clunky screen shares.
* **The Solution:** Add `devx state dump`. This command securely snapshots the running topology, failing container logs, and redacted `devx.yaml` state into a structured diagnostic report. A future V2 can add upload to GitHub Gist or internal pastebin.
* **Key files:** `cmd/state_dump.go`

### 42. Local CI Pipeline Emulation
* **Priority:** 🟡 P2
* **Effort:** High
* **Impact:** High — every developer wants this, nobody has built it well. The "fix ci" commit loop is a universal pain point and `act` is the closest competitor but riddled with environment parity issues. Even 80% parity would be valuable. However, maintaining a runner image becomes a perpetual ops tax that must be budgeted for.
* **The Problem:** Developers often have to commit and push 10 times ("fix ci", "fix ci again") to debug GitHub Actions because tools like `act` are clunky or lack exact environment parity.
* **The Solution:** `devx ci run` natively parses `.github/workflows/` and executes the steps inside the isolated `devx` Podman machine, perfectly replicating the GitHub Ubuntu runner environment and injecting necessary vault secrets so pipelines can be debugged 100% locally.

### 43. Smart File Syncing (Zero-Rebuild Hot Reloading)
* **Priority:** 🟡 P2
* **Effort:** Very High
* **Impact:** High — the pain is real (VirtioFS on macOS is genuinely slow with large trees), but building a custom file-sync daemon that reliably handles rename cascades, symlinks, `.gitignore` rules, and permission mapping across host→VM→container is a *massive* engineering effort. Evaluate embedding Mutagen as a dependency before writing from scratch. File sync bugs are silent data corruptors — ship it buggy and you destroy developer trust.
* **The Problem:** Hot reloading using OS-level volume mounts on macOS (via VirtioFS or similar) is catastrophically slow for thousands of files (e.g., `node_modules`), while rebuilding full container images disrupts developer flow state.
* **The Solution:** Implement an intelligent, lightweight sync daemon inspired by Skaffold. When a developer saves a file, `devx` natively injects the diff directly into the running Podman container, bypassing the kernel mount entirely for instant hot-reload. Consider wrapping Mutagen as the sync engine to avoid reimplementing edge cases.
* **Key files:** `cmd/sync.go`, `internal/sync/daemon.go`

---

## 🟡 P3 — Backlog (Conditional / Needs Prerequisites)

### 44. Unified Multirepo Orchestration
* **Priority:** 🟡 P3
* **Effort:** High
* **Impact:** Medium — the `include` directive from Compose is a good reference, but relative path resolution, conflicting port definitions, secret scoping, and "who owns the network?" questions make this treacherous. Takes 2 weeks to build, 6 months to get right. **Prerequisite:** Idea 34 must ship first — without dependency graphs, multi-repo is chaos squared.
* **The Problem:** Running a company's entire infrastructure often means juggling 10 different repository directories. `devx.yaml` only orchestrates what's in the current folder.
* **The Solution:** Introduce an `include` directive (borrowed from modern Compose) that allows a parent `devx.yaml` to reference and spool up configurations from relative paths in sibling repositories, creating a localized master-node orchestrator without Kubernetes.

### 45. Predictive Background Pre-Building
* **Priority:** 🟡 P3
* **Effort:** Medium
* **Impact:** Conditional — if builds take 2 seconds, pre-building saves nothing. If builds take 60 seconds, this is transformative. Value is highly dependent on the user's specific stack. Instrument build times first (via telemetry) and only invest if data shows builds are a top-3 friction point.
* **The Problem:** Even with fast builds, typing "restart" and waiting 15 seconds to rebuild a Docker image breaks the "Inner Development Loop" flow state.
* **The Solution:** `devx` runs a silent file-watcher. The moment a critical dependency definition changes (like `go.mod` or `package.json`), `devx` pre-builds the heavy image layers in the background. When the user manually issues a restart, the cache is instantly primed, returning the container completely immediately.
* **Key files:** `internal/build/watcher.go`

### 46. Hybrid Edge-to-Local Cloud Routing
* **Priority:** 🟡 P3
* **Effort:** Very High
* **Impact:** Niche — this is essentially Telepresence reimplemented inside `devx`. Telepresence itself has gone through multiple rewrites and is now commercial. Traffic interception requires either a cluster-side agent (breaking the "Client-Side Only" design principle) or DNS-level routing tricks. Genuinely useful for ~10% of developers debugging cross-service staging issues, but huge build for a narrow audience.
* **The Problem:** Complex bugs sometimes only happen with real staging integration data. Running all 50 microservices locally is impossible, but testing a local fix against the remote cluster is tedious.
* **The Solution:** Similar to Telepresence, `devx bridge` securely intercepts traffic from a specific microservice in a remote K8s staging environment and tunnels it perfectly down into the developer's local `devx` container to allow real-time cross-boundary testing.

### 47. Time-Travel Debugging (Full State Checkpoints) (DONE)
* **Priority:** 🟢 DONE
* **Effort:** High
* **Impact:** Marginal over existing `db snapshot` — CRIU is powerful but notoriously finicky (fails on containers with open network sockets, GPU state, or certain kernel features). Podman's checkpoint support is labeled experimental on macOS. The already-shipped `db snapshot` covers 80% of the use case. Consider marketing `db snapshot` more aggressively instead.
* **The Problem:** You are writing an integration test and the database gets mutated in an erroneous or complex way, forcing you to completely seed the DB again to reproduce the bug.
* **The Solution:** Expanding on `db snapshot`, `devx state checkpoint` snapshots the entire topology's RAM, volumes, and running processes using Podman checkpoints (CRIU), letting a user seamlessly "rewind" all containers back exactly 5 minutes prior to the failure.

### 48. Seed Data Runner
* **Priority:** 🟡 P3
* **Effort:** Low (reframed)
* **Impact:** Medium — schema-aware auto-generation *sounds* magical but breaks on custom ENUMs, cross-table business logic constraints, geographic/temporal data, and FK cycles. Reframed as a *runner* for user-provided seed scripts rather than an auto-generator. `devx db seed` executes a user-defined seed command against a running database, handling connection wiring automatically.
* **The Problem:** Maintaining massive `.sql` seed data files is annoying. Often local environments suffer from lack of realistic, high-volume test data.
* **The Solution:** `devx db seed` runs a user-defined seed command (configured in `devx.yaml`) against the local database. `devx` handles the connection wiring, container exec, and ensures the database is healthy before running. A future V2 could layer on schema-aware generation for simple cases.

---

## 🔴 Cut or Rethink — Not Recommended

> These ideas are either already solved by existing features, violate `devx` design principles, or target the wrong audience. They are preserved here for historical context.

### ~~49. Automatic IDE Debugger Generation~~
* **Priority:** 🔴 Cut
* **Verdict:** Solved by Dev Containers. The Dev Containers spec already handles `launch.json` generation via `customizations.vscode`. JetBrains has similar support via `.idea/runConfigurations`. Building custom IDE config generation means maintaining knowledge of every IDE's debug configuration format forever.
* **Alternative:** Improve `devcontainer.json` template quality in `devx scaffold` instead.

### ~~50. Native Network Interception & Chaos Engineering~~
* **Priority:** 🔴 Cut
* **Verdict:** Overlaps heavily with Feature 8 (Traffic Shaping & Fault Injection) already shipped. Feature 8 already does latency injection, packet dropping, and 3G simulation via `internal/trafficproxy`. HTTP response code rewriting is a nice *incremental addition* to Feature 8, not a standalone feature.
* **Alternative:** Extend Feature 8 with `devx tunnel expose --fault-inject=stripe:500` HTTP-level fault rules.

### ~~51. Automated Resource Optimization Profiler~~
* **Priority:** 🔴 Cut
* **Verdict:** eBPF requires Linux kernel ≥4.15 and doesn't work on macOS natively. Would need to run profiling *inside* the Podman VM (Fedora CoreOS minimal, likely no BPF tooling). Meanwhile, `podman stats` already gives live CPU/memory per container.
* **Alternative:** Ship `devx stats` as a prettier Bubble Tea TUI wrapper around `podman stats` with threshold alerts.

### ~~52. Distributed Event Dead-Letter Inspector~~
* **Priority:** 🔴 Cut
* **Verdict:** Too niche. Only useful for teams running Kafka/RabbitMQ locally, which is rare. Most event-driven teams test against managed cloud queues (SQS, Cloud Pub/Sub) or use the emulators already supported via `devx cloud spawn`. Building a universal DLQ inspector across Kafka, RabbitMQ, *and* SQS is massive surface area for a narrow audience.
* **Revisit condition:** If demand emerges organically, consider as a plugin/extension.

### ~~53. Device Battery & CPU Starvation Simulation~~
* **Priority:** 🔴 Cut
* **Verdict:** Solves a frontend/mobile problem with a backend tool. CPU throttling via cgroups works for *server-side* containers, but the write-up frames this as simulating low-end phones. Frontend race conditions on phones are caused by JS event loop starvation, GPU compositing limits, and touch event timing — none of which are simulated by limiting CPU on a Linux container. Chrome DevTools' built-in CPU throttling is the right tool.
* **Alternative:** None — this is a browser DevTools concern, not a `devx` concern.

### ~~54. Instant "PR Preview" Cloud Ejection~~
* **Priority:** 🔴 Cut
* **Verdict:** Directly violates the "Client-Side Only Architecture" design principle from PRODUCT_ANALYSIS.md. The moment `devx` ships local state to a cloud provider, you own: cloud credentials, billing, container registries, DNS, SSL, teardown TTLs, data residency compliance, and "who pays the Fly.io bill?" Corporate security teams will reject this instantly. This is what Vercel/Netlify/Render preview deploys exist for.
* **Alternative:** None — let deployment platforms own this complexity. `devx` should stay local-first.
