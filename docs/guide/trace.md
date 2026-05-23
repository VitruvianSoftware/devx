# Shift-Left Distributed Observability

When running multiple microservices locally natively or via `devx.yaml`, figuring out *where* a request failed often requires tailing multiple sets of logs. Full distributed tracing is traditionally reserved for cloud/production environments due to the friction of setting up an OTLP collector and trace backends locally.

`devx` brings observability to your local environment with zero configuration.

## Architecture & Execution Flow

Below are the architectural component structure and the step-by-step execution flow of Shift-Left Distributed Observability.

### Component Diagram (C4 Level 2)

```mermaid
graph TD
    subgraph Host ["Developer Host / Environment"]
        cli["devx CLI"]
        envInjector["OTEL Env Injector"]
        devxShell["devx shell"]
    end

    subgraph Runtime ["Container Runtime (Podman / Docker)"]
        jaeger["Jaeger All-in-One\n(UI: :16686, OTLP: :4317)"]
        grafana["Grafana LGTM Stack\n(Loki + Grafana + Tempo + Mimir)"]
        grafanaUI["Grafana UI (:3000)"]
        tempoUI["Tempo (:3100)"]
        dashboard["Auto-Provisioned\nBuild Metrics Dashboard"]
    end

    subgraph AppServices ["Application Services"]
        svc1["Service A\n(OTEL SDK)"]
        svc2["Service B\n(OTEL SDK)"]
    end

    subgraph Storage ["Local Persistence (optional)"]
        telemetryDir["~/.devx/telemetry/"]
    end

    cli -->|"devx trace spawn"| jaeger
    cli -->|"devx trace spawn grafana"| grafana
    grafana --- grafanaUI
    grafana --- tempoUI
    grafana --- dashboard

    cli -->|"injects OTEL_EXPORTER_OTLP_ENDPOINT"| envInjector
    envInjector -->|"env var"| devxShell
    envInjector -->|"env var"| svc1
    envInjector -->|"env var"| svc2

    svc1 -->|"OTLP traces"| jaeger
    svc2 -->|"OTLP traces"| jaeger
    svc1 -->|"OTLP traces"| grafana
    svc2 -->|"OTLP traces"| grafana

    grafana -->|"--persist"| telemetryDir
    jaeger -->|"--persist"| telemetryDir

    cli -->|"devx trace list / rm"| jaeger
    cli -->|"devx trace list / rm"| grafana
```

### Execution Lifecycle Flowchart

```mermaid
flowchart TD
    Start(["devx trace spawn [engine]"]) --> SelectEngine{{"Engine argument?"}}

    SelectEngine -->|"(none / jaeger)"| Jaeger["Select Jaeger All-in-One image"]
    SelectEngine -->|"grafana"| Grafana["Select Grafana LGTM image"]

    Jaeger --> PersistCheck
    Grafana --> ProvisionDashboard["Auto-provision Build Metrics dashboard"]
    ProvisionDashboard --> PersistCheck

    PersistCheck{{"--persist flag?"}} -->|"Yes"| MountVolume["Mount ~/.devx/telemetry/ as volume"]
    PersistCheck -->|"No"| Ephemeral["Run with ephemeral storage"]

    MountVolume --> StartContainer
    Ephemeral --> StartContainer

    StartContainer["Start container via runtime"] --> ContainerOK{{"Container started?"}}
    ContainerOK -->|"No"| Error["❌ Print error (port conflict, runtime not found)"]
    Error --> Done([Done])

    ContainerOK -->|"Yes"| InjectEnv["Set OTEL_EXPORTER_OTLP_ENDPOINT\nfor devx shell and managed containers"]
    InjectEnv --> PrintUI["Print UI URL\n(Jaeger :16686 / Grafana :3000)"]
    PrintUI --> Ready([Ready — traces flow automatically])

    Ready -.->|"later"| Manage(["devx trace list / rm"])
    Manage --> ListOrRm{{"list or rm?"}}
    ListOrRm -->|"list"| ListBackends["Show active telemetry engines"]
    ListOrRm -->|"rm"| Teardown["Stop and remove container"]
    ListBackends --> Done
    Teardown --> Done
```

## Spawning a Trace Backend

You can instantly spin up a lightweight OpenTelemetry backend locally:

```bash
# Default: runs the lightweight jaegertracing/all-in-one backend
devx trace spawn

# Or use the full Grafana LGTM (Loki, Grafana, Tempo, Mimir) stack
devx trace spawn grafana
```

### Data Persistence

Trace data is ephemeral by default, meaning deleting the container loses your traces. If you are comparing performance over time or doing benchmarking, you can persist the data locally:

```bash
devx trace spawn grafana --persist
```

This ensures trace data is durably stored in `~/.devx/telemetry/<engine>/` even across container restarts.

## Automatic Environment Injection

`devx` automatically maps the correct endpoints. Whenever a trace backend is running, `devx shell` and any managed containers will automatically have the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable injected.

This means standard OpenTelemetry SDKs running in your applications will beam traces directly to your local backend out-of-the-box, without editing your `.env` files.

## Visualizing Traces

### Jaeger

If you spawned `jaeger`, the UI is available at `http://localhost:16686`. 

![Jaeger Trace UI](/images/jaeger-trace.png)

### Grafana Tempo

If you spawned `grafana`, the UI is available at `http://localhost:3100` (Default login: admin/admin).

To find your traces:
1. Navigate to **Explore** (Compass icon) in the left sidebar.
2. Select **Tempo** from the top data sources dropdown.
3. Use the Search tab to view recent traces.

![Grafana Tempo Trace Detail](/images/grafana-tempo.png)

## devx Build Metrics Dashboard

When you spawn the Grafana backend, `devx` automatically provisions a **Build Metrics** dashboard that visualizes your CI performance from `devx agent ship`:

```bash
devx trace spawn grafana
```

The dashboard is accessible at `http://localhost:3000/d/devx-build-metrics/devx-build-metrics` and includes:

| Panel | Description |
|-------|-------------|
| Total Builds | Count of all build events |
| P50/P90 Build Time | Percentile latency from Tempo spans |
| Build Duration Over Time | Time series showing build duration trends |
| Recent Agent Ship Preflights | Table with stack, branch, and pass/fail outcomes |
| Test Details | Table containing individual granular test spans (Go only) |
| Test/Lint/Build Results | Bar Gauges showing pass/fail/skip breakdown |

![Grafana devx Build Metrics Dashboard](/images/grafana-build-metrics.png)

### Dashboard Verification

Here is an automated verification of the Build Metrics dashboard in action:

![Grafana devx Build Metrics Verification](/images/grafana-build-metrics-recording.webp)

Each `devx agent ship` and `devx run` execution exports an enriched span. `devx agent ship` pre-flights attach attributes like `devx.stack`, `devx.branch`, `devx.test.pass`, `devx.lint.pass`, and `devx.build.pass`. Furthermore, `devx run -- go test` emits individual test spans with `devx.test.name` and `devx.test.status`. All data is queryable directly in Tempo's TraceQL metrics explorer.

::: tip DOGFOODING
We use this dashboard ourselves during `devx` development — every commit to the `devx` CLI generates build metrics that we monitor to catch performance regressions.
:::

## Managing Backends

You can list running backends and remove them when finished:

```bash
# List all active telemetry engines
devx trace list

# Teardown the backend
devx trace rm jaeger
```
