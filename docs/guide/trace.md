# Shift-Left Distributed Observability

When running multiple microservices locally natively or via `devx.yaml`, figuring out *where* a request failed often requires tailing multiple sets of logs. Full distributed tracing is traditionally reserved for cloud/production environments due to the friction of setting up an OTLP collector and trace backends locally.

`devx` brings observability to your local environment with zero configuration.

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

## Managing Backends

You can list running backends and remove them when finished:

```bash
# List all active telemetry engines
devx trace list

# Teardown the backend
devx trace rm jaeger
```
