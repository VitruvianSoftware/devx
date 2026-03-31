package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/devcontainer"
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

	// Apply environment variables
	for k, v := range cfg.ContainerEnv {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Forward ports
	for _, port := range cfg.ForwardPorts {
		args = append(args, "-p", fmt.Sprintf("%d:%d", port, port))
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
