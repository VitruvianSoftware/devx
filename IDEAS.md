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

## 🟡 P3 — Backlog (Conditional / Needs Prerequisites)


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
