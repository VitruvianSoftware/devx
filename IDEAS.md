# Future Enhancements & IDEAS

This document tracks upcoming feature ideas, requests, and architectural plans for `devx`. 

When an idea is fully implemented and shipped, it is **migrated to `FEATURES.md`** to maintain an organized historical record of capabilities.

---

## Idea Template

To propose a new feature, copy the template below and add it to the appropriate priority section. Please ensure you increment the Idea Number sequentially (the last shipped feature was 64).

```markdown
### [Idea Number]. [Feature Title]
* **Priority:** 🟢 P0 | 🟢 P1 | 🟡 P2 | 🟡 P3 | 🔴 Cut
* **Effort:** Trivial | Low | Medium | High | Very High
* **Impact:** [Short impact statement]
* **The Problem:** [Describe the workflow friction, missing capability, or developer pain point]
* **The Solution:** [Describe how `devx` will solve this seamlessly. Mention potential commands, UI (TUI/logs), flags, or architecture]
* **Key files:** [Optional: List core files or packages to be added or modified]
```

---

## 🟢 Active Ideas



### 58. Remote Audit via K8s Job (`devx audit --remote`)
* **Priority:** 🟡 P3
* **Effort:** Medium
* **Impact:** Enables security scanning in CI pipelines and air-gapped environments without local container runtimes.
* **The Problem:** `devx audit` currently requires a local container runtime (podman/docker/nerdctl) to run Trivy and Gitleaks when they're not natively installed. In CI runners or environments where a K8s cluster is available but no local daemon exists, audit cannot execute. Additionally, some teams want centralized vulnerability scanning with shared Trivy DB caches to avoid redundant downloads.
* **The Solution:** Add a `ModeKubernetes` execution path alongside `ModeNative` and `ModeContainer`. When `--remote` is passed (or when the provider is `cluster`), `devx audit` creates an ephemeral K8s Job using the scanner image, transfers the source tree into the pod via `kubectl cp` or tar-pipe (`tar cf - . | kubectl exec -i <pod> -- tar xf - -C /scan`), streams logs back to the terminal, and cleans up the Job on exit. A shared PVC can optionally cache the Trivy CVE database across runs. **Not recommended for pre-push hooks** due to pod scheduling latency (15-45s) — designed for CI/CD pipelines (`devx agent ship`, GitHub Actions) where the code is already cluster-adjacent.
* **Key files:** `internal/audit/k8s.go`, `cmd/audit.go`

### 59. Intelligent Failure Recovery (AI-Enhanced Error Handling)
* **Priority:** 🟢 P1
* **Effort:** Medium
* **Impact:** Eliminates the most common developer frustration — cryptic container failures — across every `devx` command.
* **The Problem:** When `devx up` or any command fails, developers get a raw error message and an exit code. They then have to manually inspect containers, read logs, check port bindings, and cross-reference environment variables to figure out what went wrong. This is the single highest-friction moment in the `devx` workflow.
* **The Solution:** Bake automatic failure analysis into every `devx` command — not as a separate subcommand, but as a recovery layer that triggers on any non-zero exit. When a command fails, `devx` automatically collects the full runtime context (container inspect, logs, port bindings, env vars, topology from `devx.yaml`) and diagnoses the root cause.
  * **Without LLM:** Pattern-match the exit code and stderr against a built-in knowledge base of common failures (e.g., "password authentication failed" → check `.env` mismatch, "address already in use" → show `lsof` output for the port). Display the matched rule and suggested fix command.
  * **With LLM:** Feed the full runtime context graph to the AI for a precise, contextual diagnosis that understands the relationships between services. Example: "Your `api` container can't connect to `db` because the `POSTGRES_PASSWORD` in api's env is `dev123` but the db container was spawned with `devpass` from `.env.local`."
  * **Design constraint:** The command must never print "Error: no AI provider found." The rule-based fallback is the baseline; the LLM is an enhancement activated via `--explain` or automatically when a provider is detected.
* **Key files:** `internal/ai/diagnose.go`, `internal/devxerr/recovery.go`, `cmd/up.go` (and other command files for hook integration)

### 60. Natural Language Database Queries (`devx db ask`)
* **Priority:** 🟢 P1
* **Effort:** Low (schema extraction infrastructure already exists from Idea 57)
* **Impact:** Eliminates context-switching to GUI database tools (DBeaver, TablePlus) for quick local data inspection.
* **The Problem:** During development, developers constantly need to check local database state — "did that migration run?", "what does the user record look like?", "are there orphaned rows?" Each time they either write raw SQL, open a GUI tool, or ask their coding agent to generate a query. All three options require knowing the schema or switching context away from the terminal.
* **The Solution:** Add `devx db ask <engine> "<question>"` that translates natural language to SQL using the schema already extractable via `pg_dump`/`mysqldump` (infrastructure built for Idea 57), executes it against the running container, and displays results as a formatted terminal table.
  * **Without LLM:** Provide a library of canned diagnostic queries accessible via named shortcuts: `devx db ask postgres --recent` (last 10 rows from each table), `devx db ask postgres --sizes` (table sizes), `devx db ask postgres --missing-indexes` (tables without indexes), `devx db ask postgres --nulls <table>` (columns with high NULL ratios). These cover the most common "quick check" workflows without any AI.
  * **With LLM:** Full natural language → SQL translation. Example: `devx db ask postgres "users who signed up this week but never placed an order"`.
  * **Safety:** All generated queries run inside a read-only transaction (`SET TRANSACTION READ ONLY`). Mutations are blocked unless `--allow-writes` is explicitly passed. `--dry-run` shows the generated SQL without executing.
* **Key files:** `cmd/db_ask.go`, `internal/database/query.go`, `internal/ai/text2sql.go`

### 61. Test Generation from Intercepted Traffic (`devx test generate`)
* **Priority:** 🟡 P2
* **Effort:** High
* **Impact:** Generates real-world test cases from actual service traffic, catching edge cases that hand-written tests miss.
* **The Problem:** Writing integration tests is tedious and developers tend to only test the happy path. Meanwhile, `devx bridge intercept` already captures real HTTP request/response pairs flowing between services during local development — that traffic data is currently discarded after the intercept session ends.
* **The Solution:** Record intercepted traffic and transform it into test functions. Since `devx` sits in the network path via `bridge intercept`, it has unique access to real service-to-service communication that no other CLI tool can observe.
  * **Without LLM:** Generate boilerplate test scaffolding from intercepted request/response shapes — assert HTTP status codes, response headers, content-type, and JSON key presence. Output a compilable test file with `TODO` markers where intelligent assertions would go. This alone saves significant time on test setup boilerplate.
  * **With LLM:** Generate intelligent, nuanced assertions — validate business logic in response bodies, generate table-driven test cases with edge case variations, add meaningful test names that describe the behavior being verified.
  * **Supported languages:** Go (`*_test.go`), Python (`test_*.py`) — auto-detected from project structure.
  * **Usage:** `devx test generate --from-intercept api --lang go --output ./tests/`
* **Key files:** `cmd/test_generate.go`, `internal/testing/recorder.go`, `internal/testing/codegen.go`, `internal/ai/testgen.go`

### 62. Schema Drift Analysis (`devx db review`)
* **Priority:** 🟡 P2
* **Effort:** Medium
* **Impact:** Catches dangerous migration risks (data loss, constraint violations) before they reach production.
* **The Problem:** Database migrations are one of the highest-risk changes a developer can make. A dropped column, a narrowed type, or a removed index can cause data loss or performance regressions. Currently, developers eyeball migration SQL and hope they didn't miss anything. Code review catches logic bugs but rarely catches subtle schema risks like "this column has 50,000 non-null rows and you're about to drop it."
* **The Solution:** `devx db review` compares the current running database schema against a baseline (either migration files or a previous snapshot from `devx db snapshot`) and reports drift with risk assessment.
  * **Without LLM:** Produce a deterministic schema diff — added/removed/altered tables, columns, indexes, and constraints. Flag high-risk operations with built-in rules: dropping columns with data, narrowing column types, removing indexes on large tables. Output as a structured report or `--json` for CI integration.
  * **With LLM:** Enhance the diff with plain-English risk explanations: "Dropping `legacy_id` from `orders` — this column has a foreign key reference from `refunds.order_legacy_id` which will cause a constraint violation. Consider adding a migration step to update `refunds` first."
  * **CI integration:** `devx db review --ci` returns exit code 0 for safe migrations, exit code 87 for risky ones. Designed to gate PR merges.
* **Key files:** `cmd/db_review.go`, `internal/database/schemadiff.go`, `internal/ai/schemaexplain.go`

### 63. Smart Topology Suggestions (`devx init --suggest`)
* **Priority:** 🟡 P3
* **Effort:** Low
* **Impact:** Reduces onboarding friction for new projects — `devx.yaml` writes itself.
* **The Problem:** When a developer runs `devx init` on an existing project, they have to manually write every service entry in `devx.yaml` — figuring out which databases, caches, and message brokers their application needs, which ports to map, and which environment variables to set. This is error-prone and requires reading through dependency files and source code.
* **The Solution:** Scan the project's dependency manifests (`go.mod`, `package.json`, `requirements.txt`, `Gemfile`) and source imports to detect infrastructure dependencies, then suggest `devx.yaml` topology entries.
  * **Without LLM:** Use a hardcoded dependency → service mapping table: `github.com/lib/pq` or `pg` → suggest PostgreSQL container, `github.com/go-redis/redis` or `ioredis` → suggest Redis container, `kafka-go` or `kafkajs` → suggest Kafka via `devx cloud spawn`. Generate a well-commented `devx.yaml` with the detected services pre-filled and sensible defaults.
  * **With LLM:** Read deeper into source files to detect non-obvious dependencies (e.g., a Stripe webhook handler that needs a `devx mock up stripe` entry, or an S3 client that should map to a MinIO container). Generate smarter environment variable defaults based on actual usage patterns in the code.
  * **Usage:** `devx init --suggest` during initial project setup, or `devx config suggest` to analyze an existing project.
* **Key files:** `cmd/init.go`, `internal/suggest/detect.go`, `internal/suggest/rules.go`, `internal/ai/topology.go`

---

## 🔴 Cut or Rethink — Not Recommended

> These ideas are either already solved by existing features, violate `devx` design principles, or target the wrong audience. They are preserved here for historical context.

### ~~49. Automatic IDE Debugger Generation~~
* **Priority:** 🔴 Cut
* **Verdict:** Solved by Dev Containers. The Dev Containers spec already handles `launch.json` generation via `customizations.vscode`. JetBrains has similar support via `.idea/runConfigurations`. Building custom IDE config generation means maintaining knowledge of every IDE's debug configuration format forever.
* **Alternative:** Improve `devcontainer.json` template quality in `devx scaffold` instead.

### ~~50. Native Network Interception & Chaos Engineering~~
* **Priority:** 🔴 Cut
* **Verdict:** Overlaps heavily with Feature 8 (Traffic Shaping & Fault Injection) already shipped. Feature 8 already does latency injection, packet dropping, and 3G simulation via `internal/trafficproxy`. HTTP response code rewriting is a nice *incremental addition* to Feature 8, not a standalone feature.
* **Alternative:** Extend Feature 8 with `devx tunnel expose --fault-inject=stripe:500` HTTP-level fault rules.

### ~~51. Automated Resource Optimization Profiler~~
* **Priority:** 🔴 Cut
* **Verdict:** eBPF requires Linux kernel ≥4.15 and doesn't work on macOS natively. Would need to run profiling *inside* the Podman VM (Fedora CoreOS minimal, likely no BPF tooling). Meanwhile, `podman stats` already gives live CPU/memory per container.
* **Alternative:** Ship `devx stats` as a prettier Bubble Tea TUI wrapper around `podman stats` with threshold alerts.

### ~~52. Distributed Event Dead-Letter Inspector~~
* **Priority:** 🔴 Cut
* **Verdict:** Too niche. Only useful for teams running Kafka/RabbitMQ locally, which is rare. Most event-driven teams test against managed cloud queues (SQS, Cloud Pub/Sub) or use the emulators already supported via `devx cloud spawn`. Building a universal DLQ inspector across Kafka, RabbitMQ, *and* SQS is massive surface area for a narrow audience.
* **Revisit condition:** If demand emerges organically, consider as a plugin/extension.

### ~~53. Device Battery & CPU Starvation Simulation~~
* **Priority:** 🔴 Cut
* **Verdict:** Solves a frontend/mobile problem with a backend tool. CPU throttling via cgroups works for *server-side* containers, but the write-up frames this as simulating low-end phones. Frontend race conditions on phones are caused by JS event loop starvation, GPU compositing limits, and touch event timing — none of which are simulated by limiting CPU on a Linux container. Chrome DevTools' built-in CPU throttling is the right tool.
* **Alternative:** None — this is a browser DevTools concern, not a `devx` concern.

### ~~54. Instant "PR Preview" Cloud Ejection~~
* **Priority:** 🔴 Cut
* **Verdict:** Directly violates the \"Client-Driven Architecture\" design principle from PRODUCT_ANALYSIS.md. The moment `devx` ships local state to a cloud provider, you own: cloud credentials, billing, container registries, DNS, SSL, teardown TTLs, data residency compliance, and "who pays the Fly.io bill?" Corporate security teams will reject this instantly. This is what Vercel/Netlify/Render preview deploys exist for.
* **Alternative:** None — let deployment platforms own this complexity. `devx` should stay local-first.
