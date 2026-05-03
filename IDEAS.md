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

## 🟢 Active Ideas

### 55. Instant PR Sandboxing (`devx preview`)
* **Priority:** 🟢 P1
* **Effort:** High
* **Impact:** Eliminates context-switching friction during PR reviews.
* **The Problem:** Reviewing a PR locally destroys flow state. You have to stash changes, checkout the branch, run migrations, and spin up dependencies.
* **The Solution:** A command (`devx preview <PR_NUMBER>`) that automatically creates a temporary Git worktree for the PR, reads its `devx.yaml`, spins up isolated ephemeral databases for it, and exposes the app on a unique tunnel URL—all without touching your active Git branch.
* **Key files:** `cmd/preview.go`, `internal/preview/sandbox.go`

### 56. Peer-to-Peer State Replication (`devx state share` & `attach`)
* **Priority:** 🟡 P2
* **Effort:** High
* **Impact:** Drastically reduces "works on my machine" debugging time.
* **The Problem:** "It doesn't work on my machine." Helping a teammate means trying to manually replicate their database state and environment variables.
* **The Solution:** Bundle the live memory state, the database snapshot, and the specific container image SHAs into an encrypted, portable artifact. A developer runs `devx state share` to get a unique ID, and a teammate runs `devx state attach <ID>` to instantly boot the exact broken environment.
* **Key files:** `cmd/state_share.go`, `cmd/state_attach.go`, `internal/state/replication.go`

### 57. AI-Driven Synthetic Data Generation (`devx db synthesize`)
* **Priority:** 🟡 P3
* **Effort:** Medium
* **Impact:** Catches edge-case bugs by generating highly realistic, chaotic data.
* **The Problem:** Enterprises lock down prod data, and manual seed scripts generate "perfect" data that misses edge-case bugs (weird Unicode names, missing fields, extreme lengths).
* **The Solution:** Leverage the Local AI Bridge (`devx ai spawn`). The developer runs `devx db synthesize --engine postgres --records 1000`. `devx` parses the schema, passes it to the local LLM, and streams back highly realistic, chaotic synthetic SQL inserts directly into the local container.
* **Key files:** `cmd/db_synthesize.go`, `internal/database/synthesizer.go`

### 58. Remote Audit via K8s Job (`devx audit --remote`)
* **Priority:** 🟡 P3
* **Effort:** Medium
* **Impact:** Enables security scanning in CI pipelines and air-gapped environments without local container runtimes.
* **The Problem:** `devx audit` currently requires a local container runtime (podman/docker/nerdctl) to run Trivy and Gitleaks when they're not natively installed. In CI runners or environments where a K8s cluster is available but no local daemon exists, audit cannot execute. Additionally, some teams want centralized vulnerability scanning with shared Trivy DB caches to avoid redundant downloads.
* **The Solution:** Add a `ModeKubernetes` execution path alongside `ModeNative` and `ModeContainer`. When `--remote` is passed (or when the provider is `cluster`), `devx audit` creates an ephemeral K8s Job using the scanner image, transfers the source tree into the pod via `kubectl cp` or tar-pipe (`tar cf - . | kubectl exec -i <pod> -- tar xf - -C /scan`), streams logs back to the terminal, and cleans up the Job on exit. A shared PVC can optionally cache the Trivy CVE database across runs. **Not recommended for pre-push hooks** due to pod scheduling latency (15-45s) — designed for CI/CD pipelines (`devx agent ship`, GitHub Actions) where the code is already cluster-adjacent.
* **Key files:** `internal/audit/k8s.go`, `cmd/audit.go`

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
* **Verdict:** Directly violates the \"Client-Driven Architecture\" design principle from PRODUCT_ANALYSIS.md. The moment `devx` ships local state to a cloud provider, you own: cloud credentials, billing, container registries, DNS, SSL, teardown TTLs, data residency compliance, and "who pays the Fly.io bill?" Corporate security teams will reject this instantly. This is what Vercel/Netlify/Render preview deploys exist for.
* **Alternative:** None — let deployment platforms own this complexity. `devx` should stay local-first.
