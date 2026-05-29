# Service Orchestration & Dependency Graphs

`devx up` acts as a full-fledged Directed Acyclic Graph (DAG) service orchestrator, offering capabilities on par with Docker Compose and Skaffold, but deeply integrated into the `devx` VM, Tunnel, and Tailscale ecosystem.

By defining `services` in `devx.yaml`, `devx` manages the entire startup sequence of your application topology. It eliminates "Connection Refused" loops by intelligently gating trailing services behind robust HTTP/TCP health checks.

## Defining Services in `devx.yaml`

Services map directly to applications running locally on your laptop or inside the `devx` VM. 

```yaml
services:
  - name: api
    runtime: host
    command: ["go", "run", "./cmd/api"]
    port: 8080
    depends_on:
      - name: postgres
        condition: service_healthy
      - name: redis
        condition: service_healthy
    healthcheck:
      http: "http://localhost:8080/health"
      interval: "2s"
      timeout: "30s"
      retries: 3
    env:
      LOG_LEVEL: debug
```

### Flexibility via Runtimes

The `runtime` parameter gives development teams ultimate execution flexibility:
* `host` (Default): Runs the process natively on your machine via standard execution (e.g. `npm run dev`).
* `bridge`: Connects to a remote Kubernetes service via `bridge_target` (outbound port-forward) or `bridge_intercept` (inbound traffic steal). Fully managed by the DAG with correct dependency ordering and lifecycle. See [Bridge Guide](bridge.md#hybrid-topology).
* `container`: (Coming soon) Runs the process isolated within a defined sandbox.
* `kubernetes`: Renders your manifests (`kustomize`, `raw`, or `helm`) and deploys them to a target cluster, readiness-gating the DAG on the workload becoming `Available`. Can also build + load local images first. The skaffold-class deploy path. See [Kubernetes Deploy](kubernetes-deploy.md).
* `cloud`: Deploys the service to **Google Cloud Run** via `gcloud run deploy` — records the deploy, surfaces the URL, and removes it on shutdown. The skaffold-class Cloud Run deploy path. See [Cloud Run Deploy](cloud-run.md).

---

## Startup Sequence (DAG) Execution

When running `devx up`, dependencies are resolved and grouped into parallel execution tiers.

### Component Diagram (C4 Level 2)

```mermaid
graph TD
    subgraph CLI ["User / Agent Interface"]
        user["User / AI Agent"]
        cli["devx CLI"]
    end

    subgraph Core ["Orchestration Engine (internal/orchestrator)"]
        config["Config Parser & Profiles Manager"]
        dag["DAG Scheduler & Resolver"]
        portres["Port Conflict Resolver"]
        diag["Diagnostics Engine (Crash logs)"]
    end

    subgraph NodeTypes ["Runner Nodes (DAG execution)"]
        hostnode["HostNode (Local Subprocesses)"]
        bridgenode["BridgeNode (K8s Tunnels & Intercepts)"]
        kubenode["KubeNode (Helm / Kubectl manifests)"]
        cloudnode["CloudRunNode (Cloud Run services)"]
    end

    user -->|"runs devx up"| cli
    cli --> config
    config -->|"parses devx.yaml + overrides"| dag
    dag -->|"resolves ports"| portres
    dag -->|"tier execution"| NodeTypes
    NodeTypes -->|"on failure"| diag
```

### Execution Lifecycle Flowchart

```mermaid
flowchart TD
    Start([devx up]) --> Config[Parse devx.yaml & Merge Profile Overrides]
    Config --> ValidateGraph{Circular dependencies?}
    ValidateGraph -- Yes --> Fail[Fail: Circular Graph Error]
    ValidateGraph -- No --> ResolvePorts[Resolve Port Conflicts & Auto-Shift]
    ResolvePorts --> BuildTiers[Build Execution Tiers 1..N]
    
    BuildTiers --> LoopTiers{For each Tier}
    LoopTiers -->|"Next Tier"| StartTier[Execute all tier nodes in parallel]
    
    StartTier --> WaitNode{Wait for readiness condition}
    
    WaitNode -- service_healthy --> PollHealth[HTTP/TCP Healthcheck Polling]
    WaitNode -- service_completed_successfully --> WaitOneShot[Wait for exit 0 of task]
    
    PollHealth --> CheckNodeStatus{Status ok?}
    WaitOneShot --> CheckNodeStatus
    
    CheckNodeStatus -- Yes --> AllNodesReady{All tier nodes ready?}
    CheckNodeStatus -- No / Timeout --> TailLogs[Extract last 50 lines of logs & Show Crash Card]
    TailLogs --> TearDownStarted[Tear down already started tiers in reverse order]
    TearDownStarted --> Fail
    
    AllNodesReady -- Yes --> MoreTiers{More Tiers?}
    AllNodesReady -- No --> WaitNode
    
    MoreTiers -- Yes --> LoopTiers
    MoreTiers -- No --> Running([All services running & healthy])
    
    Running --> CtrlC[Shutdown Triggered / Ctrl+C]
    CtrlC --> ReverseTiers[Stop services in reverse tier order N..1]
    ReverseTiers --> End([Cleanup Completed])
```

### Service Lifecycle States

```mermaid
stateDiagram-v2
    [*] --> Pending
    Pending --> WaitingOnDeps : dependencies not ready
    WaitingOnDeps --> Starting : all deps healthy
    Pending --> Starting : no dependencies
    Starting --> HealthChecking : process spawned
    HealthChecking --> Healthy : healthcheck passes
    HealthChecking --> Failed : healthcheck timeout / retries exhausted
    Healthy --> ShuttingDown : Ctrl+C / devx down
    Failed --> ShuttingDown : cleanup triggered
    ShuttingDown --> Stopped
    Stopped --> [*]
```
 

![DAG Execution Output Screenshot] 
```text
$ devx up
🏗️ Bootstrapping Project 'demo-app' Databases...
🚀 Spawning postgres on port 5432...

✅ postgres is running!
  Container:  devx-db-postgres
  Connection: postgres://postgres:password@127.0.0.1:5432/postgres

📋 Starting tier 1: api, worker
  🚀 Starting api: go run ./cmd/api
  🚀 Starting worker: go run ./cmd/worker
  ⏳ Waiting for api to become healthy...
  ✅ api is healthy

📋 Starting tier 2: web
  🚀 Starting web: npm run dev
  ⏳ Waiting for web to become healthy...
  ✅ web is healthy

✅ All services are running and healthy.

🎉 All services are now explicitly available worldwide! Press Ctrl+C to stop
```

---

## One-shot tasks (run-to-completion)

Not every node is a long-running server. Data seeds, schema migrations, and other init steps need to **run once, finish, and gate whatever depends on them**. Mark such a node `oneshot: true`: devx runs its command, waits for the process to exit, and treats **exit 0** as "ready" — instead of polling a healthcheck. Dependents express the wait with `condition: service_completed_successfully`.

```yaml
services:
  # Populate the local object store, then exit.
  - name: seed
    runtime: host                       # or container
    command: ["node", "seed-local.mjs"]
    oneshot: true
    depends_on:
      - name: fake-gcs
        condition: service_healthy
    healthcheck:
      timeout: "5m"                     # optional: bound the run (default 10m)

  # Doesn't start until the seed has completed successfully.
  - name: offer
    runtime: host
    command: ["npm", "run", "dev:offer"]
    port: 8080
    depends_on:
      - name: seed
        condition: service_completed_successfully
```

```text
📋 Starting tier 2: seed
  🚀 Starting seed: node seed-local.mjs
  ⏳ Waiting for seed to complete...
  ✅ seed completed

📋 Starting tier 3: offer
  🚀 Starting offer: npm run dev:offer
  ✅ offer is healthy
```

Semantics:

* **Success = exit 0.** A non-zero exit fails `devx up` (with the task's crash logs tailed inline), so dependents never start against a half-prepared environment.
* **Bounded.** A one-shot is given `healthcheck.timeout` to finish (default 10m); exceeding it is treated as a failure.
* **Tier-ordered.** Because dependents `depends_on` the task, it lands in an earlier tier — the next tier only starts once it has completed.

This is the local-dev counterpart to a Kubernetes seed `Job`: in a [`runtime: kubernetes`](kubernetes-deploy.md) deploy the seed ships as a `Job` inside your manifests, while `oneshot` covers the `host`/`container` path.

---

## Visual Architecture Map (`devx map`)

For onboarding engineers, reading a 300-line `devx.yaml` file to understand the system topology is daunting. `devx` provides a command to visualize the orchestration DAG immediately:

```bash
devx map
```

This parses the internal routing, container bounds, network ports, and `devx.yaml` dependencies to instantly output a [Mermaid.js](https://mermaid.js.org/) flowchart diagram. You can pipe this into an output file for architecture documentation:

```bash
devx map --output topology.md
```

---

## Environment Profiles (`--profile`)

Skaffold and Docker Compose natively understand that a local workflow isn't a monolith. A frontend developer might not need a local Kafka queue, while a backend developer might not need the local React build.

`devx` introduces **Environment Profiles** directly into `devx.yaml`. Profiles allow developers to apply additive or merging configuration overlays to the base setup with a simple flag:

```bash
devx up --profile backend-only
```

In `devx.yaml`, define conditional blocks:

```yaml
profiles:
  backend-only:
    services:
      - name: api
        env:
          LOG_LEVEL: trace
      - name: kafka-consumer
        runtime: host
        command: ["go", "run", "./cmd/consumer"]
        depends_on:
          - name: postgres
            condition: service_healthy
```

**Override Semantics:**
- **Matching names**: fields are merged (the profile wins over the base config).
- **New entries**: appended contextually (adding a new service to the execution DAG).
- **Omitted fields**: inherited from the base `devx.yaml`.

---

## Automatic Port Conflict Resolution

Nothing breaks developer flow state worse than `EADDRINUSE`. 

`devx` automatically checks if defined ports are available natively before attempting to bind them. If a rogue ghost process is occupying your desired port (e.g. `:8080`), `devx` will:
1. Automatically negotiate a free OS port (e.g. `:51939`).
2. Shift the application targeting that port.
3. Subtly rewrite all Cloudflare Tunnel `targetPort` configurations so external routing never breaks.
4. Inject the overridden port into the service's environment as `PORT=51939` so the framework dynamically binds to the correct port.

![Port Shift Warning Screenshot]
```text
$ devx up
🏗️ Bootstrapping Project 'demo-app' Databases...

⚠️  Port 5432 is already in use — auto-shifted to port 51939.
   If your application hardcodes port 5432 (e.g., DATABASE_URL=...:5432),
   it will NOT connect. Use the devx-injected environment variables
   ($PORT, $DB_PORT, $DATABASE_URL) instead of static values.

🚀 Spawning postgres on port 51939...
```

---

## Context-Aware Crash Diagnostics

If a container fails to start, or a native hosted service crashes during the startup sequence, `devx` traps the exit error and automatically retrieves the final 50 tailing lines.

The crash output is cleanly presented in a high-contrast box, eliminating the need to search through separate log files.

![Crash Log Tailing Screenshot]
```text
$ devx up
📋 Starting tier 1: api
  🚀 Starting api: go run ./cmd/api
  ⏳ Waiting for api to become healthy...

💥 api (host) crashed — last log output:
╭──────────────────────────────────────────────────╮
│  2026/04/02 17:33:00 Booting service...          │
│  2026/04/02 17:33:01 Checking database link      │
│  panic: fatal missing required ENCRYPTION_KEY    │
│                                                  │
│  goroutine 1 [running]:                          │
│  main.initDB()                                   │
│      /app/cmd/api/main.go:23 +0x65               │
╰──────────────────────────────────────────────────╯

Error: healthcheck failed for "api": timed out after 30s
```
