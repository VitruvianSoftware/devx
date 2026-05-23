# Container VMs

The `devx vm` commands manage the lifecycle of your local development VM — a lightweight Linux instance running inside your chosen provider (Lima, Colima, Docker, OrbStack, or Podman).

## Architecture & Execution Flow

Below are the architectural component structure and the step-by-step execution flow of the Container VM feature.

### Component Diagram (C4 Level 2)

```mermaid
graph TD
    subgraph Host ["Developer Host / Environment"]
        cli["devx CLI"]
        butane["Butane Compiler"]
        resolver["Provider Resolver"]
        envFile[".env Credentials"]
    end

    subgraph Provider ["VM Provider (Lima / Colima / Docker / OrbStack / Podman)"]
        vm["VM Instance"]
    end

    subgraph VM ["Inside VM (Linux Guest)"]
        runtime["Container Runtime"]
        cf_svc["cloudflared.service (systemd)"]
        ts_svc["tailscaled.service (systemd)"]
        volumes["Persistent Volumes"]
    end

    subgraph External ["External Services"]
        cfEdge["Cloudflare Edge"]
        tsCoord["Tailscale Coordination Server"]
    end

    cli -->|"compiles Butane templates"| butane
    butane -->|"produces Ignition config"| resolver
    cli -->|"injects credentials from"| envFile
    resolver -->|"provisions VM via detected provider"| vm
    vm -->|"boots systemd units"| cf_svc
    vm -->|"boots systemd units"| ts_svc
    vm -->|"mounts"| volumes
    vm -->|"starts"| runtime
    cf_svc -->|"establishes tunnel"| cfEdge
    ts_svc -->|"registers node"| tsCoord
```

### Execution Lifecycle Flowchart

```mermaid
flowchart TD
    Start(["devx vm init"]) --> DetectProvider{"--provider flag\nspecified?"}
    DetectProvider -->|"Yes"| UseFlag["Use specified provider"]
    DetectProvider -->|"No"| AutoDetect["Auto-detect provider\n(Lima → Colima → Docker → OrbStack → Podman)"]
    AutoDetect --> ProviderFound{"Provider\nfound?"}
    ProviderFound -->|"No"| ErrNoProvider["❌ Error: no supported\nVM provider detected"]
    ProviderFound -->|"Yes"| UseFlag
    UseFlag --> CompileButane["Compile Butane templates\ninto Ignition config"]
    CompileButane --> DryRun{"--dry-run\nflag set?"}
    DryRun -->|"Yes"| PrintIgnition["Print Ignition JSON\nto stdout"] --> ExitDry(["Exit (no VM created)"])
    DryRun -->|"No"| CreateVM["Create VM via provider\nwith Ignition file"]
    CreateVM --> VMCreated{"VM creation\nsucceeded?"}
    VMCreated -->|"No"| ErrCreate["❌ Error: VM creation failed"]
    VMCreated -->|"Yes"| WaitBoot["Wait for systemd\nservices to boot"]
    WaitBoot --> BootOk{"Boot completed\nwithin timeout?"}
    BootOk -->|"No"| ErrTimeout["❌ Error: boot timeout"]
    BootOk -->|"Yes"| VerifyCF["Verify Cloudflare Tunnel\nconnectivity"]
    VerifyCF --> CFOk{"Tunnel\nconnected?"}
    CFOk -->|"No"| ErrTunnel["❌ Error: Cloudflare Tunnel\nfailed to connect"]
    CFOk -->|"Yes"| VerifyTS["Verify Tailscale\nconnectivity"]
    VerifyTS --> TSOk{"Tailscale\nconnected?"}
    TSOk -->|"No"| ErrTS["❌ Error: Tailscale\nfailed to connect"]
    TSOk -->|"Yes"| Success(["✅ VM ready"])
```

### VM Lifecycle States

```mermaid
stateDiagram-v2
    [*] --> Uninitialized
    Uninitialized --> Provisioning : devx vm init
    Provisioning --> Running : boot complete
    Provisioning --> Failed : boot error
    Running --> Running : devx vm resize
    Running --> Sleeping : devx vm sleep
    Running --> Destroyed : devx vm teardown
    Sleeping --> Running : devx vm init (wake)
    Sleeping --> Destroyed : devx vm teardown
    Failed --> Destroyed : devx vm teardown
    Destroyed --> [*]
```

## Commands

### `devx vm init`

Provisions a new VM with Cloudflare Tunnel and Tailscale pre-configured.

```bash
devx vm init
```

This is the only command most developers need to run. It:

1. Compiles a Butane config into Ignition format
2. Creates a VM using your selected provider with the Ignition file
3. Starts the VM and waits for systemd services to boot
4. Verifies Cloudflare Tunnel and Tailscale connectivity

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--provider` | `auto-detect` | Backend: `lima`, `colima`, `podman`, `docker`, or `orbstack` |
| `--dry-run` | — | Preview the Ignition config without creating the VM |

### `devx vm status`

Shows the health of all three components:

```bash
devx vm status
```

```
┌──────────────────────────────────────────────────────
│  📊 VM Status
├──────────────────────────────────────────────────────
│  VM:               ✅ running
│  Cloudflare:       ✅ connected (tunnel-id: abc123)
│  Tailscale:        ✅ connected (100.x.x.x)
└──────────────────────────────────────────────────────
```

### `devx vm resize`

Dynamically adjust VM resources without reprovisioning:

```bash
devx vm resize --cpus 4 --memory 8192
```

### `devx vm ssh`

Drop into an SSH shell inside the VM:

```bash
devx vm ssh
```

### `devx vm sleep` / `devx vm sleep-watch`

Pause the VM to free resources, or run a background daemon that auto-sleeps idle VMs:

```bash
devx vm sleep              # Pause now
devx vm sleep-watch        # Auto-sleep after idle timeout
```

### `devx vm teardown`

Stop and remove the VM. This is a **destructive** operation and will prompt for confirmation:

```bash
devx vm teardown
```

## Ignition Configuration

The VM is configured using a [Butane](https://coreos.github.io/butane/) file that compiles to Ignition format. This config:

- Installs and starts `tailscaled` and `cloudflared` as systemd units
- Sets kernel parameters (`fs.inotify.max_user_watches`, `fs.aio-max-nr`)
- Configures persistent volumes for container data
- Injects credentials from your `.env` file

The Butane templates are stored in `internal/ignition/` and compiled at `devx vm init` time.
