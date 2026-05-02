# VM Providers

`devx` is intentionally VM-agnostic. While it relies on an underlying container runtime to execute the isolated Linux environment, you have complete freedom to choose the virtualization backend that best fits your workflow, hardware, and OS.

By decoupling the CLI from a hardcoded backend, `devx` supports five major providers natively: **Lima**, **Colima**, **Docker Desktop**, **OrbStack**, and **Podman**.

## Choosing a Provider

### 1. Lima
- **Best for:** Most macOS and Linux users seeking a lightweight, fast, and open-source backend.
- **Runtime:** `nerdctl`
- **Installation:** `brew install lima`
- **Details:** Lima creates lightweight, automatic Linux virtual machines. `devx` configures it with optimized file sharing (VirtioFS) to ensure your local directories sync instantly into the VM.

### 2. Colima
- **Best for:** Developers who want Lima's architecture but prefer a more pre-configured "batteries-included" setup.
- **Runtime:** `nerdctl` / `docker`
- **Installation:** `brew install colima`
- **Details:** Built on top of Lima, Colima is an excellent open-source alternative that provides Docker and Containerd compatibility with minimal setup.

### 3. Docker Desktop
- **Best for:** Corporate environments where Docker Desktop is already provisioned or developers deeply integrated into the Docker ecosystem.
- **Runtime:** `docker`
- **Installation:** `brew install --cask docker`
- **Details:** If Docker Desktop is running, `devx` will natively hook into its daemon. It bypasses creating a custom VM entirely and directly leverages Docker's hypervisor.

### 4. OrbStack
- **Best for:** Performance-obsessed macOS users.
- **Runtime:** `docker`
- **Installation:** `brew install --cask orbstack`
- **Details:** OrbStack is a wildly fast, drop-in replacement for Docker Desktop on macOS. `devx` interacts with OrbStack just as it would with Docker, yielding significantly faster boot times and lower memory overhead.

### 5. Podman
- **Best for:** Advanced users needing explicit daemonless containers, immutable Fedora CoreOS features, or time-travel debugging (CRIU).
- **Runtime:** `podman`
- **Installation:** `brew install podman`
- **Details:** Podman Machine boots a dedicated Fedora CoreOS VM. This provider is the only one that supports advanced `devx state checkpoint` and `devx state restore` capabilities because of CRIU integration.

---

## Configuration Hierarchy

`devx` automatically detects your installed providers. If multiple providers are detected, it prompts you to choose. It stores and resolves your preference through a strict hierarchy:

1. **CLI Override:** `devx vm init --provider=lima`
2. **Project-Local Config (`devx.yaml`):**
   ```yaml
   name: my-project
   provider: colima
   ```
3. **Machine-Local Config:** Saved globally in `~/.devx/config.yaml`.
4. **Auto-Detection:** Prompts the user if multiple backends are detected without a saved preference.

## Provider Abstraction (`ContainerRuntime`)

Regardless of the provider chosen, the `devx` CLI injects a unified `ContainerRuntime` interface across the entire toolset. 

This means commands like `devx exec`, `devx nuke`, `devx db spawn`, and the internal log multiplexers automatically adapt their binary execution (e.g., executing `nerdctl rm` instead of `podman rm`) seamlessly without requiring you to change your commands.
