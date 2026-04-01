package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/VitruvianSoftware/devx/internal/ai"
	"github.com/VitruvianSoftware/devx/internal/devcontainer"
	"github.com/VitruvianSoftware/devx/internal/envvault"
)

var shellProviderFlag string

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Launch an isolated dev shell from devcontainer.json",
	Long: `Reads the devcontainer.json from your project, pulls the container image,
mounts your workspace, applies environment variables, and drops you into
an interactive shell with all your project's tooling pre-configured.`,
	RunE: runShell,
}

func init() {
	shellCmd.Flags().StringVar(&shellProviderFlag, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	rootCmd.AddCommand(shellCmd)
}

func runShell(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting cwd: %w", err)
	}

	if err := ensureVMRunning(); err != nil {
		return err
	}

	cfg, cfgPath, err := devcontainer.Load(cwd)
	if err != nil {
		return fmt.Errorf("devcontainer: %w", err)
	}

	fmt.Printf("📦 Found %s\n", cfgPath)

	if cfg.Image == "" {
		return fmt.Errorf("devcontainer.json must specify an 'image' field")
	}

	runtime := shellProviderFlag
	if runtime != "docker" && runtime != "podman" {
		return fmt.Errorf("unsupported runtime %q — use 'podman' or 'docker'", runtime)
	}

	// Build the container run arguments
	containerName := fmt.Sprintf("devx-shell-%s", filepath.Base(cwd))
	workspaceFolder := cfg.WorkspaceFolder
	if workspaceFolder == "" {
		workspaceFolder = "/workspace"
	}

	args := []string{
		"run", "--rm", "-it",
		"--name", containerName,
		"-v", fmt.Sprintf("%s:%s", cwd, workspaceFolder),
		"-w", workspaceFolder,
	}

	// Auto-mount the devx binary so the full CLI is available inside the container
	if devxBin, err := os.Executable(); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/usr/local/bin/devx:ro", devxBin))
		fmt.Println("🔧 devx CLI mounted into container at /usr/local/bin/devx")
	}

	// Share host network so devx tunnel commands work seamlessly from inside.
	// This gives the container direct access to the host's network stack,
	// which is critical for tunnel commands to work consistently across
	// all providers (podman, docker, orbstack).
	args = append(args, "--network", "host")

	// Apply environment variables from devcontainer.json
	for k, v := range cfg.ContainerEnv {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Dynamic secret injection from devx.yaml (1password, gcp, bitwarden, etc.)
	userSecrets := make(map[string]string)
	aiBridgeEnabled := true // Enabled by default
	if yamlData, err := os.ReadFile("devx.yaml"); err == nil {
		type devxConfig struct {
			Env []string `yaml:"env"`
			AI  struct {
				Bridge *bool `yaml:"bridge"`
			} `yaml:"ai"`
		}
		var devxCfg devxConfig
		_ = yaml.Unmarshal(yamlData, &devxCfg)
		
		if devxCfg.AI.Bridge != nil && !*devxCfg.AI.Bridge {
			aiBridgeEnabled = false
		}
		
		// "mix and match which ones, multiple, or none which will default back to using plain .env"
		if len(devxCfg.Env) == 0 {
			devxCfg.Env = []string{"file://.env"}
		}

		if len(devxCfg.Env) > 0 {
			if secrets, sErr := envvault.PullAll(devxCfg.Env); sErr == nil {
				for k, v := range secrets {
					userSecrets[k] = v
					args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
				}
				fmt.Printf("🔒 Injected %d secrets from remote vaults seamlessly.\n", len(secrets))
			} else {
				fmt.Fprintf(os.Stderr, "⚠️ Warning: Failed to fetch secrets from vault: %v\n", sErr)
			}
		}
	} else if _, sErr := os.Stat(".env"); sErr == nil {
		// Fallback straight to .env if devx.yaml doesn't even exist
		if secrets, err := envvault.PullAll([]string{"file://.env"}); err == nil {
			for k, v := range secrets {
				userSecrets[k] = v
				args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
			}
			fmt.Printf("🔒 Injected %d secrets from .env natively.\n", len(secrets))
		}
	}

	// AI Bridge: Discover native host LLMs and inject into the container unless overridden.
	if aiBridgeEnabled {
		if aiEnv := ai.DiscoverHostLLMs(runtime); aiEnv.Active {
			injectedAny := false
			for k, v := range aiEnv.EnvVars {
				// Prevent overwriting if the user explicitly defined this variable in .env or a remote vault
				if _, userOverridden := userSecrets[k]; !userOverridden {
					args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
					injectedAny = true
				}
			}
			if injectedAny {
				fmt.Printf("🤖 Local AI Detected (%s).\n", aiEnv.EngineName)
				fmt.Printf("↳ Injected OPENAI_API_BASE / ANTHROPIC_BASE_URL to route local agents (claude, opencode, codex)\n")
				fmt.Printf("  to your host engine. To disable this override, set 'ai: bridge: false' in devx.yaml.\n")
			}
		}
	}

	// Auto-inject env vars for any running devx cloud emulators.
	// This means 'devx shell' just works out of the box if you've run
	// 'devx cloud spawn gcs' — no manual .env editing required.
	if cloudEnvs := discoverCloudEmulatorEnvs(runtime); len(cloudEnvs) > 0 {
		for k, v := range cloudEnvs {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
		fmt.Printf("☁️  Injected %d cloud emulator endpoints (STORAGE_EMULATOR_HOST, etc.)\n", len(cloudEnvs))
	}

	// Auto-inject SMTP_HOST / SMTP_PORT / MAIL_CATCHER_URL if MailHog is running.
	if mailEnvs := discoverMailEnvVars(runtime); len(mailEnvs) > 0 {
		for k, v := range mailEnvs {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
		fmt.Printf("📬 Injected mail catcher endpoints (SMTP_HOST, SMTP_PORT, MAIL_CATCHER_URL)\n")
	}

	// Note: port forwarding (-p) is intentionally skipped when using host
	// networking. The host network mode already exposes all container ports
	// directly, and combining -p with --network host causes warnings or
	// errors on Docker Desktop and OrbStack.

	// Bridge core configurations and advanced agent workflows into the container.
	// We resolve the correct home directory based on remoteUser to ensure
	// consistent behavior across all runtimes and user configurations.
	if home, err := os.UserHomeDir(); err == nil {
		containerHome := "/root"
		if cfg.RemoteUser != "" && cfg.RemoteUser != "root" {
			containerHome = fmt.Sprintf("/home/%s", cfg.RemoteUser)
		}

		// 1. Cloudflare Credentials
		cfDir := filepath.Join(home, ".cloudflared")
		if _, statErr := os.Stat(cfDir); statErr == nil {
			args = append(args, "-v", fmt.Sprintf("%s:%s/.cloudflared:ro", cfDir, containerHome))
		}

		// 2. Global Git Configuration (enables agents to push/pull via HTTP)
		gitCfg := filepath.Join(home, ".gitconfig")
		if _, statErr := os.Stat(gitCfg); statErr == nil {
			args = append(args, "-v", fmt.Sprintf("%s:%s/.gitconfig:ro", gitCfg, containerHome))
		}

		// 3. AI / Agent Identity dotfiles (RW as agents frequently refresh tokens)
		agentPaths := []string{
			".claude.json",
			".config/claude",
			".config/opencode",
			".config/gemini-cli",
			".codex",
			".openai",
			".agent",      // Global skills vault
			".gemini",     // Common gemini state / antigravity
		}

		mountedAgents := 0
		for _, p := range agentPaths {
			hostPath := filepath.Join(home, p)
			if _, statErr := os.Stat(hostPath); statErr == nil {
				args = append(args, "-v", fmt.Sprintf("%s:%s/%s", hostPath, containerHome, p))
				mountedAgents++
			}
		}
		if mountedAgents > 0 {
			fmt.Printf("🧠 Ported %d agent identities & skill vaults natively into the workspace.\n", mountedAgents)
		}
	}

	// 4. SSH Agent forwarding (enables agents to authenticate against Git over SSH)
	if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		if _, statErr := os.Stat(sshAuthSock); statErr == nil {
			// Mount the socket file identically so it's queryable at the same path 
			// and inject the matching environment variable.
			args = append(args, "-v", fmt.Sprintf("%s:%s", sshAuthSock, sshAuthSock))
			args = append(args, "-e", fmt.Sprintf("SSH_AUTH_SOCK=%s", sshAuthSock))
			fmt.Println("🔑 Forwarded SSH_AUTH_SOCK securely to enable agent operations.")
		}
	}

	// 5. Docker Socket for Agentic Sandboxing (Docker-in-Docker)
	// Checks default host locations for sockets that could allow agents to spin up containers.
	// We map it generically to /var/run/docker.sock to appease most open-source CLI agents.
	dockerSockets := []string{"/var/run/docker.sock", filepath.Join(os.Getenv("HOME"), ".docker/run/docker.sock")}
	for _, sock := range dockerSockets {
		if sock != "" {
			if _, statErr := os.Stat(sock); statErr == nil {
				args = append(args, "-v", fmt.Sprintf("%s:/var/run/docker.sock", sock))
				fmt.Println("🐳 Mounted docker.sock for AI Agent sandboxing privileges.")
				break
			}
		}
	}

	// Set remote user
	if cfg.RemoteUser != "" {
		args = append(args, "--user", cfg.RemoteUser)
	}

	args = append(args, cfg.Image)

	// If there's a postCreateCommand, run it then drop into shell
	postCmd := cfg.PostCreateCmd()
	if postCmd != "" {
		args = append(args, "sh", "-c", fmt.Sprintf("%s && exec sh", postCmd))
	} else {
		args = append(args, "sh")
	}

	fmt.Printf("🚀 Launching %s shell (%s)...\n", cfg.Name, runtime)
	if cfg.Name == "" {
		fmt.Printf("🚀 Launching dev shell (%s)...\n", runtime)
	}

	// Print useful info
	fmt.Printf("   Image:     %s\n", cfg.Image)
	fmt.Printf("   Workspace: %s → %s\n", cwd, workspaceFolder)
	if len(cfg.ForwardPorts) > 0 {
		ports := make([]string, len(cfg.ForwardPorts))
		for i, p := range cfg.ForwardPorts {
			ports[i] = fmt.Sprintf("%d", p)
		}
		fmt.Printf("   Ports:     %s\n", strings.Join(ports, ", "))
	}
	if postCmd != "" {
		fmt.Printf("   Post-init: %s\n", postCmd)
	}
	fmt.Println()

	cmd := exec.Command(runtime, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil && strings.Contains(err.Error(), "signal: interrupt") {
		return nil
	}
	return err
}
