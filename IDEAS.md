# Future Enhancements & IDEAS

This document tracks upcoming feature ideas, requests, and architectural plans for `devx`. 

When an idea is fully implemented and shipped, it is **migrated to `FEATURES.md`** to maintain an organized historical record of capabilities.

---

## Idea Template

To propose a new feature, copy the template below and add it to the **Proposed Ideas** section. Please ensure you increment the Idea Number sequentially (the last shipped feature was 26).

```markdown
### [Idea Number]. [Feature Title]
* **The Problem:** [Describe the workflow friction, missing capability, or developer pain point]
* **The Solution:** [Describe how `devx` will solve this seamlessly. Mention potential commands, UI (TUI/logs), flags, or architecture]
* **Key files:** [Optional: List core files or packages to be added or modified]
```

---

## Proposed Ideas

### 27. Service Scaffolding & Internal Developer Platforms (IDPs)
* **The Problem:** `devx doctor` sets up the prerequisite tools, but getting a new microservice off the ground (frameworks, Dockerfiles, linting, CI config) requires manually copying from other repos, slowing down new development.
* **The Solution:** Implement `devx scaffold <template>`. Using a community or internal Template Registry, instantly generate a paved-path repository pre-wired with standard pipelines, Tailscale connectivity, and `devx.yaml` configurations.
* **Key files:** `cmd/scaffold.go`, `internal/scaffold/engine.go`

### 28. Zero-Friction Local AI Inference
* **The Problem:** Developers building AI applications struggle with the overhead of running models locally (configuring Ollama, vLLM, passing through GPU drivers) to avoid massive OpenAI API costs during dev/test loops.
* **The Solution:** Implement `devx ai spawn`. Automatically handles downloading, caching, and serving models (via `ollama` or similar containerized inference engines) inside the VM with proper GPU acceleration logic mapped. Auto-injects `OPENAI_API_BASE` into `devx shell` to route SDKs locally.
* **Key files:** `cmd/ai.go`, `internal/ai/inference.go`

### 29. Shift-Left Distributed Observability
* **The Problem:** When running 5 microservices locally via `devx.yaml`, figuring out *where* a request failed requires tailing 5 sets of logs. Full distributed tracing is currently reserved for cloud/production because setting up an OTLP collector + Jaeger locally is too tedious.
* **The Solution:** Implement `devx trace` or `devx observe`. Instantly spins up a lightweight OpenTelemetry Collector and Grafana Alloy/Jaeger stack. Auto-injects `OTEL_EXPORTER_OTLP_ENDPOINT` into all managed containers. Provides a beautiful terminal dashboard or local web UI links to visualize spans.
* **Key files:** `cmd/trace.go`, `internal/telemetry/otel.go`

### 30. Ephemeral E2E Browser Testing Environments
* **The Problem:** Writing and running Cypress or Playwright tests locally destroys the developer's local database state or fights with active ports, breaking their flow state.
* **The Solution:** Implement `devx test ui`. Boots an entirely cloned, perfectly clean topology (app + fresh cloned DB) in isolation, runs headless browser integration tests against it, then immediately tears it down. 
* **Key files:** `cmd/test_ui.go`, `internal/testing/ephemeral.go`

### 31. Unified OpenAPI & 3rd-Party Mocking
* **The Problem:** If Stripe, Twilio, or an internal downstream team's API goes down, local development is completely blocked.
* **The Solution:** Implement `devx mock`. Parses an `openapi.yaml` spec and spins up a local mock server (like Prism or WireMock) under a `*.devx.local` domain, instantly providing fake generated response schemas so developers can keep coding offline.
* **Key files:** `cmd/mock.go`, `internal/mock/prism.go`

### 32. Zero-Config Local Kubernetes (Kind / k3s)
* **The Problem:** `devx` excels at standard container/VM execution, but developers shipping to Kubernetes ultimately need to test manifests, Helm charts, and operators locally without destroying their macbooks with Minikube.
* **The Solution:** Implement `devx k8s spawn`. Creates a lightning-fast `k3s` or `kind` cluster *inside* the existing Podman machine, wires it seamlessly to the Cloudflare tunnels, and automatically configures the developer's `~/.kube/config` on the host.
* **Key files:** `cmd/k8s.go`, `internal/k8s/k3s.go`
