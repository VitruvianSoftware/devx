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
* `container`: (Coming soon) Runs the process isolated within a defined sandbox.
* `kubernetes`: (Coming soon) Runs the process natively via an injected pod specification into the `devx k8s` local cluster.
* `cloud`: (Coming soon) Runs the process attached remotely to GCP Cloud Run or AWS ECS via emulators.

---

## Startup Sequence (DAG) Execution

When running `devx up`, dependencies are resolved and grouped into parallel execution tiers. 

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
