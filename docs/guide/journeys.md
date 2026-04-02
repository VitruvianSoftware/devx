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

---

## 3. The Backend Maintainer: Troubleshooting Production Data

When debugging a complex bug, you often need to trace it against *real* data without leaking PII or destroying your local setup.

### Step 1: Provision the Database
Stand up an isolated, persistent instance of the required database engine.
```bash
devx db spawn postgres --name debug-db
```

### Step 2: Pull Anonymized State
Instead of copying a raw SQL dump to your hard drive, orchestrate a secure, streaming injection of anonymized production data directly into the newly spawned database.
```bash
devx db pull postgres
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
