# Unified OpenAPI & 3rd-Party Mocking (Idea #31)

This plan details the implementation of an instant mocking framework (`devx mock`) that automatically spins up intelligent OpenAPI mock servers based on Remote URLs. This eliminates the dependency on 3rd-party SaaS sandboxes being available for local development.

## Proposed Changes

### 1. Configuration Architecture (CLI + YAML Parity)

We will extend `devx.yaml` to include a `mocks` array, while also supporting ad-hoc CLI provisioning.
*(Note: V1 will strictly pull from remote HTTP/HTTPS URLs. A `TODO` will be added in the codebase to support binding local `openapi.yaml` file paths natively in a future iteration).*

**YAML Definition:**
```yaml
# devx.yaml
mocks:
  - name: stripe
    url: https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.yaml
    port: 4010 # (Optional, defaults to next free port)
```

### 2. New Commands (`cmd/mock*.go`)

Following **Option A (Daemonized)**, mocks will spin up silently in the background alongside your databases. They remain strictly isolated from `devx up` for now to prevent bloating the default startup, though a unified lifecycle concept (like Skaffold `run`) will be researched for future iterations.

- `devx mock`: Root command namespace.
- `devx mock up [name...]`: Reads `devx.yaml` and starts persistent `stoplight/prism:5` containers for all (or specified) declared mocks.
- `devx mock list` (or `ls`): Lists running mock servers, their associated OpenAPI URLs, and Local Bindings (e.g., `MOCK_STRIPE_URL=http://localhost:4010`).
- `devx mock restart [name]`: Bounces the mock container (useful if the remote URL spec changes and needs re-fetching).
- `devx mock rm [name]`: Stops and removes the mock container.

### 3. Expanding the Database CLI (`cmd/db_restart.go`)

To ensure parity across all stateful background dev environments, the `db` command group will be updated:
- *Current*: `devx db spawn`, `devx db list`, `devx db rm`
- *New*: `devx db restart <engine>` (e.g. `devx db restart postgres`) will be implemented to gracefully stop and start existing DB containers without destroying their volumes.

### 4. Container Execution Details

- **Image:** `stoplight/prism:5`
- **Execution:** Spawns via `podman run -d --name devx-mock-<name> --label devx-mock=true -p <port>:4010 stoplight/prism:5 mock -h 0.0.0.0 <url>`
- **Tracking:** Relies on the `managed-by=devx` and `devx-mock=true` container labels for lifecycle monitoring (used by `list` and `rm`).

## Verification Plan

### Manual Verification & Documentation Proof
1. I will configure a mock in `devx.yaml` pointing to a real remote OpenAPI representation (e.g. Stripe, or a generic placeholder API).
2. I will run `devx mock up` and sequentially query the mock container using curl to demonstrate its intelligence.
3. I will run `devx mock list` to show the active daemons.
4. Using the browser subagent, I will render this terminal flow into an isolated aesthetic HTML terminal window and capture a screenshot.
5. I will embed this "Verification Proof" directly into a new `/guide/mocking` VitePress documentation page!
