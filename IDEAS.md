# Future Enhancements & IDEAS

This document outlines the top 10 planned features to make `devx` the undisputed leader in local developer productivity. These ideas focus on breaking down traditional paywalls (like those found in ngrok) and making `devx` hyper-extensible for any team.

## The "Ngrok Alternatives" (Paywall Breakers)

### 1. Local Request Inspector & Webhook Replay (TUI) (DONE)
* **The Problem:** Ngrok's web inspector allows devs to see incoming HTTP requests and manually replay them. This is crucial for webhook development but is heavily commercialized.
* **The Solution:** Build a beautiful Bubble Tea terminal UI (`devx tunnel inspect`). It acts as a local proxy that intercepts, displays, and allows one-click replaying of HTTP requests coming through the Cloudflare tunnel—right in your terminal.

### 2. Built-in Authentication & Access Control (DONE)
* **The Problem:** Putting Basic Auth, OAuth, or IP whitelisting on an exposed tunnel usually requires an expensive subscription.
* **The Solution:** Add simple flags like `devx tunnel expose 8080 --basic-auth "user:pass"` or `--github-org "VitruvianSoftware"`. Because we leverage Cloudflare, we can automatically configure Cloudflare Access API to enforce zero-trust policies dynamically on edge URLs for free.

### 3. Multi-Port Mapping via Project Configs (`devx.yaml`) (DONE)
* **The Problem:** Running multiple tunnels (e.g., frontend, backend, webhooks) requires opening multiple terminal tabs and managing them manually.
* **The Solution:** Read a `devx.yaml` file in the project root that defines a local topology. A developer runs `devx tunnel up`, and it simultaneously exposes all defined ports under unified, deterministic subdomains.

### 4. Custom Domain Support (BYOD) (DONE)
* **The Problem:** Using a custom, branded domain name (e.g., `api.mycompany.dev`) instead of a random subdomain is a premium-only feature in most tools.
* **The Solution:** Allow users to bring their own Cloudflare zones to the CLI. Pass `--domain mycompany.dev` to provision custom edges effortlessly and permanently.

---

## Developer Productivity & Extensibility

### 5. Backend Pluggability (OrbStack / Docker Support) (DONE)
* **The Problem:** Hardcoding Fedora CoreOS + Podman Machine is highly opinionated. Some devs might already have Docker Desktop or OrbStack running and don't want to run two VMs.
* **The Solution:** Abstract the `internal/podman` package into a `VirtualizationProvider` interface. Allow developers to map `devx` networking (Cloudflare + Tailscale) directly on top of their existing hypervisor (`--provider=orbstack`).

### 6. Built-in Dev Containers (`devcontainer.json`) Integration (DONE)
* **The Problem:** The VM gives you an OS, but you still need node, go, rust, etc. for your specific project.
* **The Solution:** Add native support for `devcontainer.json`. Running `devx shell` reads the config, spins up the exact container inside the Podman VM, mounts your code, and drops you into a shell with completely isolated tooling.

### 7. One-Click Database Provisioning (`devx db spawn`) (DONE)
* **The Problem:** Setting up local databases (Postgres, Redis), persisting data, and wiring them securely takes friction.
* **The Solution:** Add `devx db spawn postgres`. This spins up Postgres inside the VM, wires it to the Tailnet automatically, and prints the connection string. It guarantees persistence across VM rebuilds using core Podman volumes.

### 8. Network Simulation (Traffic Shaping & Fault Injection) (DONE)
* **The Problem:** Testing how your local app handles high latency, dropped packets, or 3G network speeds is extremely tedious.
* **The Solution:** Add `--throttle=3g` or `--latency=200ms` flags to the `expose` command. We leveraged a native Go TCP proxy (`internal/trafficproxy`) to intercept the Cloudflare tunnel and artificially shape incoming Edge traffic. This allows cross-platform simulation (macOS/Docker/OrbStack) for frontend/mobile testing without OS-level `tc` limits.

### 9. Automated Resource Scaling & Deep Sleep (DONE)
* **The Problem:** VMs reserve RAM and CPU constantly, draining Macbook batteries when sitting idle.
* **The Solution:** Implemented auto-pause/deep-sleep (`devx vm sleep-watch`) when no containers are active. We also added seamless JIT wake-ups. VM will automatically resume the moment you invoke any `devx` commands that require infrastructure. Finally, we added support to dynamically resize the VM (`devx vm resize --cpus 4 --memory 8192`) without destroying the machine context.

### 10. Global Secret Sync & `.env` Management
* **The Problem:** Sharing `.env` files across a team securely is a massive pain, often resulting in Slack DMs and out-of-sync configurations.
* **The Solution:** Integrate with a vault provider (1Password CLI, Bitwarden, or GCP Secret Manager). Running `devx config pull` fetches the secure, team-managed `.env` for the current project automatically and injects it securely without storing plaintext on the Mac's disk.
