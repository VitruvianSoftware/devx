# Local API Mocking

## The Problem

If Stripe goes down, if Twilio throttles your sandbox, or if an internal downstream team's staging API is unavailable—your local development is completely blocked. You can't test payment flows, notifications, or integrations until the dependency recovers.

## The Solution: `devx mock`

`devx mock` spins up an intelligent, **schema-faithful mock server** for any 3rd-party or internal API in seconds using a remote OpenAPI spec. Mocks run as persistent background containers alongside your databases — always available, zero cost, offline-capable.

```bash
devx mock up stripe \
  --url https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.yaml
```

The mock returns structurally correct responses generated from the OpenAPI schema, giving your application realistic data to work with without ever hitting a real API.

## Configuration

**CLI + YAML Parity**: All options are configurable both via CLI flags and `devx.yaml`.

### YAML Configuration (committed team defaults)

```yaml
# devx.yaml
mocks:
  - name: stripe
    url: https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.yaml
    port: 4010   # (Optional — defaults to next free port)

  - name: internal-payments
    url: https://internal-api.company.com/openapi.json
```

### CLI Flags (one-off use, overrides YAML)

```bash
# Start all mocks from devx.yaml
devx mock up

# Start a specific mock by name
devx mock up stripe

# Start an ad-hoc mock without devx.yaml
devx mock up stripe \
  --url https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.yaml

# Use Docker instead of Podman
devx mock up stripe --runtime docker
```

## Architecture & Execution Flow

Below are the architectural component structure and the step-by-step execution flow of Local API Mocking.

### Component Diagram (C4 Level 2)

```mermaid
graph TD
    subgraph Host ["Developer Host"]
        cli["devx CLI"]
        runtime["Container Runtime (podman / docker / nerdctl)"]
        shell["devx shell"]
    end

    subgraph MockContainer ["Prism Container (devx-mock-&lt;name&gt;)"]
        prism["Stoplight Prism Mock Engine"]
        specloader["OpenAPI Spec Loader"]
        mockendpoints["Mock HTTP Endpoints"]
    end

    subgraph Remote ["Remote Spec Source"]
        specurl["OpenAPI Spec URL (HTTPS)"]
    end

    subgraph App ["Application"]
        appservice["Your App / Service"]
    end

    cli -->|"mock up &lt;name&gt;"| runtime
    runtime -->|"pull & run stoplight/prism"| MockContainer
    specloader -->|"fetch remote spec"| specurl
    specloader -->|"parse & validate"| prism
    prism -->|"generate schema-faithful responses"| mockendpoints
    appservice -->|"HTTP requests to localhost:port"| mockendpoints
    shell -->|"inject MOCK_&lt;NAME&gt;_URL"| appservice
    cli -->|"mock rm / restart"| runtime
```

### Execution Lifecycle Flowchart

```mermaid
flowchart TD
    Start(["devx mock up &lt;name&gt;"]) --> ParseConfig["Parse devx.yaml mocks[] or CLI --url flag"]
    ParseConfig -->|No spec URL found| ErrNoSpec["Error: no OpenAPI spec URL provided"]
    ParseConfig -->|OK| DetectRuntime["Detect container runtime (--runtime or auto)"]
    DetectRuntime -->|Not found| ErrRuntime["Error: no container runtime found"]
    DetectRuntime -->|Found| CheckExisting{"devx-mock-&lt;name&gt; already running?"}
    CheckExisting -->|Yes| ReportExisting["Report existing mock (idempotent)"]
    CheckExisting -->|No| PullImage["Pull stoplight/prism image (if not cached)"]
    PullImage -->|Pull failed| ErrPull["Error: image pull failed"]
    PullImage -->|OK| BindPort["Bind local port (--port or next free port)"]
    BindPort -->|Port in use| AutoShift["Auto-shift to next free port"]
    AutoShift --> StartContainer
    BindPort -->|OK| StartContainer["Start devx-mock-&lt;name&gt; container\nwith spec URL as argument"]
    StartContainer -->|Failed| ErrStart["Error: container start failed"]
    StartContainer -->|OK| FetchSpec["Prism fetches & validates OpenAPI spec"]
    FetchSpec -->|Spec invalid / unreachable| ErrSpec["Error: spec fetch or parse failed"]
    FetchSpec -->|OK| Serve["Prism serves mock endpoints"]
    Serve --> PrintEnv["Print MOCK_&lt;NAME&gt;_URL=http://localhost:&lt;port&gt;"]
    PrintEnv --> Ready["✓ Mock server running"]

    Ready --> Restart(["devx mock restart &lt;name&gt;"])
    Restart --> StopOld["Stop existing container"]
    StopOld --> StartContainer

    Ready --> Remove(["devx mock rm &lt;name&gt;"])
    Remove --> StopRm["Stop & remove devx-mock-&lt;name&gt; container"]
    StopRm --> Done["✓ Mock removed"]
```

## How It Works

1. Pulls the `stoplight/prism` image (industry-standard OpenAPI mock engine)
2. Starts a background container (`devx-mock-<name>`) bound to a free local port
3. Prism fetches the remote OpenAPI spec and starts serving schema-faithful HTTP responses
4. Reports the `MOCK_<NAME>_URL` environment variable you can inject into your application

## Lifecycle Commands

| Command | Description |
|---------|-------------|
| `devx mock up [name]` | Start all (or named) mocks from `devx.yaml` |
| `devx mock list` | List running mock servers with ports and env vars |
| `devx mock restart <name>` | Restart a mock (re-fetches the latest spec) |
| `devx mock rm <name>` | Stop and remove a mock |

## Environment Variables Injected

Each mock automatically maps to an environment variable you can use in your application:

| Mock Name | Environment Variable | Value |
|-----------|---------------------|-------|
| `stripe` | `MOCK_STRIPE_URL` | `http://localhost:4010` |
| `internal-payments` | `MOCK_INTERNAL_PAYMENTS_URL` | `http://localhost:4011` |

## Verification Proof

The sequence below demonstrates the full lifecycle: starting a mock for the Petstore3 API, listing it, querying it for a live schema-generated response, restarting it, and removing it.

![devx mock — OpenAPI Mock Server Lifecycle Verification](/devx_mock_proof.png)

The curl response `{"id":10,"petId":198772,"quantity":7,...}` is a **real, schema-faithful response** generated by Prism from the OpenAPI spec — no real API was called.

::: tip Restart to pick up spec changes
Use `devx mock restart <name>` whenever the upstream OpenAPI spec changes. Prism re-fetches and reloads the spec on restart.
:::

::: info Local file support
V1 only supports remote HTTP/HTTPS OpenAPI spec URLs. Local file spec support (volume-mounting) is planned for a future release.
:::

::: info One mock per name
Like `devx db spawn`, `devx mock up` provisions one container per named mock. Running the same `devx mock up stripe` twice is idempotent — it will detect the running container and report it as already live.
:::
