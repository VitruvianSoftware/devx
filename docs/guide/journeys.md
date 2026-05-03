# User Journeys

`devx` is an incredibly flexible local development orchestrator that handles everything from instant public URLs to end-to-end multi-service Kubernetes clusters. 

Depending on what you are trying to achieve, your "Golden Path" through the tooling will change. Here are the most common User Journeys.

## 1. The Platform Engineer: Starting a New Project

If you are setting up a brand new microservice or repository for your team today, this is the exact path you should take. This path assumes you want to take advantage of the entire `devx` ecosystem (AI Agents, reproducible environments, automated tunnels, etc).

### Step 0: The Preflight Check
Before touching any infrastructure, instantly audit your environment to ensure you have all prerequisites and credentials.
```bash
devx doctor
```
If anything is missing, `devx doctor install` and `devx doctor auth` will automatically install and configure it for you.

### Step 1: Bootstrapping the Infrastructure
Spin up the hyper-isolated Virtual Machine and bind it to our Cloudflare and Tailscale networking stack.
```bash
devx vm init
```

### Step 2: Generating the Project
Generate a fresh, paved-path project (e.g., a `go-api` or a Node service) that comes fully pre-wired with standard CI pipelines, `.devcontainer` configurations, and `devx.yaml` defined.
```bash
devx scaffold go-api
```

### Step 3: Booting the Topologies
Change directories into your new repository and spawn the environment. Because the scaffold pre-configured the `devx.yaml` file, this automatically spawns required local databases (like Postgres) and securely punches Cloudflare tunnels through to expose your service ports.
```bash
cd my-new-api
devx up
```

### Step 4: Entering the Flow State
Drop directly into the containerized dev environment. All internal tooling, vaults, `.env` files, and local AI agent identity tokens (like `OPENAI_API_BASE` for local LLMs) are seamlessly auto-injected.
```bash
devx shell
```

---

## 2. The Frontend Developer: Ngrok Alternative

If you already have a project running locally on your Macbook, and your _only_ goal is to securely expose it to the internet so you can test webhooks (like Stripe) or preview it on your mobile device, `devx` scales down perfectly.

### Step 1: Start your App
Run your frontend application normally on your host architecture.
```bash
npm run dev
# Server running at http://localhost:3000
```

### Step 2: Expose the Port
Use `devx tunnel` to punch a secure hole through to your process. Because we leverage Cloudflare, there's no timeout limits or paywalls.
```bash
devx tunnel expose 3000 --name my-frontend
```

### Step 3: View Requests (Optional)
Open a new terminal tab and launch the local Bubble Tea inspector. This allows you to view and replay incoming requests from the tunnel directly in your terminal.
```bash
devx tunnel inspect
```

### Alternative: The Configuration Approach (`devx.yaml`)
In keeping with the **CLI + YAML Parity** design principle, you can permanently codify this tunnel in your repository so you (and your teammates) don't have to remember the CLI flags.

Create a `devx.yaml` file:
```yaml
tunnels:
  - name: my-frontend
    port: 3000
    # Optional: Basic Auth or GitHub org restrictions
    # basic_auth: "admin:supersecret"
```

Then simply run:
```bash
devx up
```
This automatically boots the tunnel exactly as if you had typed the long-form command!

---

## 3. The Backend Maintainer: Troubleshooting Production Data

When debugging a complex bug, you often need to trace it against *real* data without leaking PII or destroying your local setup.

### Step 1: Provision the Database
Stand up an isolated, persistent instance of the required database engine.
```bash
devx db spawn postgres --name debug-db
```

*Note on YAML Parity:* If the database is already defined in your `devx.yaml` under `databases: [ { name: "debug-db", engine: "postgres" } ]`, simply running `devx up` handles this for you!

### Step 2: Pull Anonymized State
Instead of copying a raw SQL dump to your hard drive, orchestrate a secure, streaming injection of anonymized production data directly into the newly spawned database.
```bash
devx db pull postgres
```

This command relies entirely on `devx.yaml` configuration to know *how* to securely fetch data. For example, `devx.yaml` might have:
```yaml
databases:
  - name: debug-db
    engine: postgres
    pull:
      command: "gcloud storage cat gs://my-anonymized-bucket/nightly.sql"
```

### Step 3: Snapshot the Clean State
Before running any tests that might corrupt this perfect data snapshot, save its exact volume state in milliseconds.
```bash
devx db snapshot create postgres debug-db "pre-test-state"
```

If you ruin the database during your debugging, simply restore it!
```bash
devx db snapshot restore postgres debug-db "pre-test-state"
```

---

## 4. The Kubernetes Engineer: Hybrid Crossover

When building a microservice that relies on a massive staging Kubernetes cluster (too large to run locally), `devx` lets you mix local containers with remote K8s services seamlessly.

### Step 1: Start a Local Cluster (Optional)
If you just need to test standard K8s manifests locally without destroying your host machine, spin up an instant, isolated control plane.
```bash
devx k8s spawn
```
This safely extracts the `kubeconfig` without corrupting your global `~/.kube/config`.

### Step 2: Bridge Outbound (Connect)
If your local standalone app needs to query a database or API running inside a remote staging cluster, you don't need to manually configure `kubectl port-forward` strings. 

Define the target in `devx.yaml` under `bridge:`, then run:
```bash
devx bridge connect
```
Your local application can now seamlessly talk to the staging database via auto-injected `BRIDGE_*` environment variables.

### Step 3: Bridge Inbound (Intercept)
If you want real traffic from the staging cluster to hit your local code so you can use a local debugger, deploy the self-healing intercept agent.
```bash
devx bridge intercept my-service --steal
```
This deploys an ephemeral agent Pod that steals traffic destined for `my-service` and tunnels it back to your local Macbook. When you exit, it automatically restores the original traffic flow.

### Alternative: The Full Hybrid Topology
If you want to orchestrate local standalone containers and remote Kubernetes bridges simultaneously, simply set `runtime: bridge` on your dependencies in `devx.yaml`.

Then run:
```bash
devx up
```
`devx` will intelligently start your local databases, establish K8s port-forwards, and intercept staging traffic in the exact correct dependency order!
