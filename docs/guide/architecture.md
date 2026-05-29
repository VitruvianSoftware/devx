# Architecture

This page is the canonical reference for how `devx` is structured — from the external systems it connects to, down to the internal package layout. Each feature area has its own dedicated guide with detailed C4 diagrams and flowcharts; this page ties them all together.

## System Context (C4 Level 1)

`devx` sits at the center of the developer's workflow, orchestrating interactions with a wide range of external systems:

```mermaid
graph TD
    dev["👤 Developer / AI Agent"]

    subgraph devx ["devx CLI"]
        core["Orchestration Engine"]
    end

    subgraph runtimes ["Container & Compute Runtimes"]
        podman["Podman / Docker / Lima / OrbStack"]
        k8s["Kubernetes Clusters (k3s / GKE / EKS)"]
        cloudrun["Google Cloud Run"]
    end

    subgraph networking ["Networking & Edge"]
        cf["Cloudflare (Tunnels + DNS API)"]
        ts["Tailscale (WireGuard Mesh)"]
    end

    subgraph secrets ["Secrets & Config"]
        bw["Bitwarden"]
        op["1Password"]
        gcpsm["GCP Secret Manager"]
    end

    subgraph ai ["AI / LLM Providers"]
        ollama["Ollama (Local)"]
        lmstudio["LM Studio (Local)"]
        openai["OpenAI API (Cloud)"]
    end

    subgraph scm ["Source Control & CI"]
        gh["GitHub (PRs, Actions, Pages)"]
    end

    subgraph observability ["Observability"]
        jaeger["Jaeger"]
        grafana["Grafana LGTM"]
    end

    dev -->|"devx up / run / shell"| core
    core -->|"create, exec, checkpoint"| podman
    core -->|"kubectl apply, helm, port-forward"| k8s
    core -->|"gcloud run deploy"| cloudrun
    core -->|"tunnel expose, DNS CNAME"| cf
    core -->|"join tailnet"| ts
    core -->|"pull / push secrets"| bw
    core -->|"pull / push secrets"| op
    core -->|"pull / push secrets"| gcpsm
    core -->|"diagnose, synthesize, db ask"| ollama
    core -->|"diagnose, synthesize, db ask"| lmstudio
    core -->|"diagnose, synthesize, db ask"| openai
    core -->|"PR, CI, Pages deploy"| gh
    core -->|"spawn trace backend"| jaeger
    core -->|"spawn trace backend"| grafana
```

## Internal Architecture (C4 Level 2)

Inside the CLI, the architecture follows a layered design: **Config Resolution → DAG Orchestration → Runtime Nodes → Subsystem Packages**.

```mermaid
graph TD
    subgraph CLI ["CLI Layer (cmd/)"]
        commands["Cobra Commands (131 files)"]
        globals["Global Flags: --json, --dry-run, -y, --detailed"]
    end

    subgraph Core ["Core Engine"]
        configResolver["Config Resolver (devxconfig.go)"]
        profiles["Profile Manager"]
        dagEngine["DAG Scheduler & Tier Resolver (orchestrator/dag.go)"]
        portResolver["Port Conflict Resolver (internal/network)"]
        diagEngine["Failure Diagnosis Engine (internal/ai)"]
    end

    subgraph RuntimeNodes ["Runtime Nodes (internal/orchestrator/)"]
        hostNode["Host Node (subprocess exec)"]
        k8sNode["Kubernetes Node (kubectl / helm / kustomize)"]
        crNode["Cloud Run Node (gcloud run deploy)"]
        bridgeNode["Bridge Node (port-forward / intercept)"]
    end

    subgraph K8sSupport ["Kubernetes Support"]
        helmPkg["Helm Renderer (helm.go)"]
        syncPkg["Pod File Sync (kubernetes_sync.go)"]
        pfPkg["Port-Forward Discovery (kubernetes_portforward.go)"]
        imagePkg["Image Build & Load (internal/image)"]
    end

    subgraph Subsystems ["Subsystem Packages (internal/)"]
        dbPkg["database — spawn, seed, pull, snapshot, synthesize, ask"]
        cloudPkg["cloud — GCS, PubSub, Firestore, MinIO emulators"]
        bridgePkg["bridge — connect, intercept, Yamux, session state"]
        statePkg["state — dump, checkpoint, restore, share, attach"]
        syncSub["sync — Mutagen file sync sessions"]
        cronPkg["cron — job registry and one-shot runner"]
        ciPkg["ci — GitHub Actions YAML parser and executor"]
        mcpPkg["mcpserver — MCP tool server for AI agents"]
        shipPkg["ship — PR-first review and auto-merge pipeline"]
    end

    subgraph Infra ["Infrastructure Packages (internal/)"]
        providerPkg["provider — Lima, Colima, Docker, OrbStack, Podman"]
        ignitionPkg["ignition — Butane/Ignition config compiler"]
        cfPkg["cloudflare — DNS + tunnel API client"]
        tsPkg["tailscale — auth + network setup"]
        envvaultPkg["envvault — secrets from Bitwarden/1Password/GCP"]
        nukePkg["nuke — cache + container scanner and destroyer"]
    end

    subgraph Crosscutting ["Cross-Cutting Concerns"]
        telemetryPkg["telemetry — OTEL spans + build metrics"]
        tuiPkg["tui — Bubble Tea interactive UI"]
        logsPkg["logs — multiplexer + DLP redaction"]
        errPkg["devxerr — typed exit codes (15–93)"]
    end

    commands --> configResolver
    configResolver --> profiles
    profiles --> dagEngine
    dagEngine -->|"tier-parallel dispatch"| RuntimeNodes
    dagEngine --> portResolver
    k8sNode --> K8sSupport
    commands --> Subsystems
    commands --> Infra
    RuntimeNodes --> Infra
    Core --> Crosscutting
```

## Runtime Dispatch

The central design pattern of `devx up` is **runtime dispatch**: each service declares a `runtime:` field that determines how it's started, health-checked, and cleaned up.

```mermaid
flowchart LR
    config["devx.yaml service definition"]
    config --> parseRuntime{"runtime: ?"}

    parseRuntime -->|"host (default)"| host["Host Node\n• Spawn subprocess\n• HTTP/TCP healthcheck\n• Signal on shutdown"]
    parseRuntime -->|"kubernetes"| k8s["Kubernetes Node\n• Render manifests\n• kubectl apply / helm upgrade\n• kubectl wait --for=Available\n• kubectl delete on cleanup"]
    parseRuntime -->|"cloud"| cloud["Cloud Run Node\n• gcloud run deploy\n• Surface URL\n• gcloud run delete on cleanup"]
    parseRuntime -->|"bridge"| bridge["Bridge Node\n• bridge_target: port-forward\n• bridge_intercept: traffic steal\n• Yamux tunnel lifecycle"]

    k8s --> renderer{"renderer: ?"}
    renderer -->|"kustomize"| kustomize["kubectl apply -k"]
    renderer -->|"raw"| raw["kubectl apply -f"]
    renderer -->|"helm"| helm["helm upgrade --install"]

    k8s --> images{"images: defined?"}
    images -->|"yes"| buildLoad["Build → Push to\nin-cluster registry\n→ then apply"]
    images -->|"no"| directApply["Apply manifests\ndirectly"]

    k8s --> syncDef{"sync: defined?"}
    syncDef -->|"yes"| liveReload["Start pod file sync\n(poll + kubectl cp)"]
    syncDef -->|"no"| noSync["No file sync"]
```

## DAG Execution Model

When `devx up` runs, services are resolved into **parallel execution tiers** based on `depends_on` declarations. Services within the same tier start concurrently; a service only begins once all of its dependencies report healthy.

```mermaid
flowchart TD
    subgraph Tier0 ["Tier 0 (No Dependencies)"]
        db["postgres\nruntime: host"]
        redis["redis\nruntime: host"]
        emulator["gcs-emulator\nruntime: host"]
    end

    subgraph Tier1 ["Tier 1 (Depends on Tier 0)"]
        api["api\nruntime: host"]
        worker["worker\nruntime: kubernetes"]
        bridge_svc["payments-api\nruntime: bridge"]
    end

    subgraph Tier2 ["Tier 2 (Depends on Tier 1)"]
        web["web\nruntime: host"]
        cron_task["cleanup\none-shot task"]
    end

    db & redis --> api
    db --> worker
    redis --> bridge_svc
    api & worker --> web
    api --> cron_task
```

The DAG engine handles:
- **Port collision resolution** — auto-shifts ports when conflicts are detected
- **Crash diagnostics** — on failure, pulls container logs and runs pattern-based diagnosis
- **Graceful shutdown** — stops services in reverse tier order on `Ctrl+C`
- **One-shot tasks** — services that run to completion and exit rather than staying alive

## CLI Command Groups

```mermaid
graph TD
    subgraph infra ["Local Infrastructure"]
        vm["vm — VM lifecycle"]
        db["db — Ephemeral databases"]
        cloud["cloud — GCP emulators"]
        nuke["nuke — Hard reset"]
    end

    subgraph k8s ["Kubernetes & Hybrid"]
        k8scmd["k8s — Zero-config k3s"]
        cluster["cluster — Multi-node clusters"]
        bridgecmd["bridge — Remote K8s access"]
        k8sdeploy["(via devx up) — Manifest deploy"]
        crdeploy["(via devx up) — Cloud Run deploy"]
    end

    subgraph net ["Networking & Edge"]
        tunnel["tunnel — Cloudflare tunnels"]
        mail["mail — Email catcher"]
        webhook["webhook — Webhook catcher"]
    end

    subgraph orch ["Orchestration & State"]
        up["up — DAG orchestrator"]
        run["run — Native process exec"]
        sync["sync — File sync (Mutagen)"]
        state["state — Diagnostics & checkpoints"]
        config["config — Vault secrets"]
        preview["preview — PR sandboxes"]
        multirepo["(via devx.yaml include:)"]
    end

    subgraph test ["Testing & Telemetry"]
        mock["mock — API mocking (Prism)"]
        testcmd["test — Ephemeral E2E"]
        audit["audit — Security scanning"]
        trace["trace — Distributed tracing"]
        doctor["doctor — Health checks"]
    end

    subgraph ci ["Pipelines & CI/CD"]
        agent["agent — Ship / review / init"]
        cicmd["ci — Local CI emulation"]
        cron["cron — Cron job testing"]
        mcp["mcp — AI agent MCP server"]
        stats["stats — Build telemetry"]
    end
```

## Internal Package Map

The `internal/` directory contains **46 packages** organized by concern:

| Layer | Packages | Purpose |
|---|---|---|
| **Orchestration** | `orchestrator`, `config` | DAG engine, config resolution, runtime node dispatch |
| **Runtime Nodes** | `orchestrator/dag.go`, `kubernetes_node.go`, `cloudrun_node.go`, `bridge_node.go`, `helm.go`, `kubernetes_sync.go`, `kubernetes_portforward.go` | Per-runtime lifecycle (start, healthcheck, cleanup) |
| **Databases** | `database` | Engine lifecycle: spawn, seed, pull, snapshot, synthesize (AI), ask (AI) |
| **Images** | `image` | Build, push, and load container images into k8s clusters via in-cluster registry |
| **Networking** | `cloudflare`, `tailscale`, `network`, `exposure`, `trafficproxy`, `authproxy` | Tunnel management, DNS, overlay networks, port allocation |
| **Bridge** | `bridge`, `bridge/agent` | Outbound connect, inbound intercept, Yamux tunnels, self-healing agent pod |
| **State** | `state`, `sync` | CRIU checkpoints, diagnostic dumps, peer-to-peer replication, Mutagen sync |
| **Secrets** | `secrets`, `envvault` | Vault provider abstraction (Bitwarden, 1Password, GCP SM), `.env` management |
| **AI** | `ai` | Two-tier failure diagnosis, LLM provider resolution (Ollama → LM Studio → OpenAI) |
| **Infrastructure** | `provider`, `ignition`, `podman`, `k8s`, `multinode`, `cloud` | VM provisioning, Butane/Ignition, container runtime abstraction, cluster management |
| **CI/CD** | `ci`, `ship`, `agent`, `preview`, `scaffold` | GitHub Actions parser, PR workflow, preview sandboxes, project scaffolding |
| **Observability** | `telemetry`, `logs`, `inspector` | OTEL spans, log multiplexing, DLP redaction, traffic inspection |
| **Testing** | `mock`, `testing`, `audit`, `cron` | Prism mock servers, ephemeral test DBs, Gitleaks/Trivy scanning, cron runner |
| **MCP** | `mcpserver`, `mcpinstall` | Model Context Protocol server exposing devx as AI agent tools |
| **UI** | `tui` | Bubble Tea interactive components (tables, spinners, prompts) |
| **Error Handling** | `devxerr` | Typed exit codes (15–93) for deterministic error signaling |
| **Maintenance** | `updater`, `nuke`, `doctor`, `prereqs`, `devcontainer` | Self-update, hard reset, health probes, prerequisite checking |

## Feature Guide Cross-Reference

Every feature area has its own dedicated guide with architectural diagrams:

| Area | Guides |
|---|---|
| **Infrastructure** | [Container VMs](virtual-machine.md) · [Providers](providers.md) · [Databases](databases.md) · [Cloud Emulators](cloud-emulators.md) · [Nuke](nuke.md) |
| **Kubernetes** | [Zero-Config k3s](kubernetes.md) · [Kubernetes Deploy](kubernetes-deploy.md) · [Multi-Node Clusters](multinode.md) · [Cloud Run](cloud-run.md) |
| **Networking** | [Hybrid Bridge](bridge.md) · [Cloudflare Tunnels](tunnels.md) · [Email Catcher](mail.md) · [Webhook Catcher](webhook.md) |
| **Orchestration** | [Service Orchestration](orchestration.md) · [Multirepo](multirepo.md) · [Vault Secrets](vaults.md) · [File Sync](sync.md) · [Native Apps & Logs](execution.md) |
| **State** | [Diagnostics & Checkpoints](state.md) · [State Replication](state-replication.md) · [PR Preview](preview.md) · [Failure Recovery](failure-recovery.md) |
| **Testing** | [API Mocking](mocking.md) · [Ephemeral E2E](testing.md) · [Security Auditing](audit.md) · [Distributed Tracing](trace.md) · [Doctor](doctor.md) |
| **Pipelines** | [Pipeline Stages](pipeline.md) · [Local CI](ci.md) · [Cron Jobs](cron.md) · [Predictive Building](caching.md) · [MCP Server](mcp.md) · [AI Agents](ai-agents.md) |

## Design Principles

1. **Declarative-first**: Everything is configured in `devx.yaml` — no imperative scripts needed.
2. **Client-driven**: No permanent server-side agents or controllers. All operations are initiated from the developer's machine.
3. **Runtime-agnostic**: The DAG orchestrator dispatches to pluggable runtime nodes. Adding a new runtime means implementing one interface.
4. **AI-native**: Every command supports `--json` for agent consumption. The MCP server exposes devx as a tool surface. Failure diagnosis is automatic.
5. **Progressive disclosure**: `devx up` is the only command most developers need. Power features (`bridge intercept`, `state share`, `ci run`) are there when you need them.
