package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/tui"
	"github.com/spf13/cobra"
)

const (
	mailContainerName = "devx-mail-catcher"
	mailSMTPPort      = 1025
	mailUIPort        = 8025
	mailImage         = "docker.io/mailhog/mailhog:latest"
)

var mailSpawnSMTPPort int
var mailSpawnUIPort   int
var mailSpawnRuntime  string

var mailSpawnCmd = &cobra.Command{
	Use:   "spawn",
	Short: "Spawn a local SMTP email catcher (MailHog)",
	Long: `Starts a MailHog SMTP catch-all server inside your devx environment.

Any email your application sends to SMTP port 1025 is captured — never
forwarded to real recipients. View all caught emails via the web UI.

Environment variables automatically injected into 'devx shell':
  SMTP_HOST=localhost
  SMTP_PORT=1025
  MAIL_CATCHER_URL=http://localhost:8025

Works with any language's SMTP library — just point it at localhost:1025.`,
	RunE: runMailSpawn,
}

func init() {
	mailSpawnCmd.Flags().IntVarP(&mailSpawnSMTPPort, "smtp-port", "p", mailSMTPPort,
		"Host port to bind for SMTP (default 1025)")
	mailSpawnCmd.Flags().IntVar(&mailSpawnUIPort, "ui-port", mailUIPort,
		"Host port to bind for the web UI (default 8025)")
	mailSpawnCmd.Flags().StringVar(&mailSpawnRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	mailCmd.AddCommand(mailSpawnCmd)
}

func runMailSpawn(_ *cobra.Command, _ []string) error {
	runtime := mailSpawnRuntime
	smtpPort := mailSpawnSMTPPort
	uiPort := mailSpawnUIPort

	// Check if already running
	checkOut, err := exec.Command(runtime, "inspect", mailContainerName,
		"--format", "{{.State.Running}}").Output()
	if err == nil && strings.TrimSpace(string(checkOut)) == "true" {
		fmt.Printf("%s MailHog is already running.\n\n", tui.IconDone)
		printMailEnvVars(smtpPort, uiPort)
		return nil
	}

	// Remove any stopped container with the same name
	_ = exec.Command(runtime, "rm", "-f", mailContainerName).Run()

	fmt.Printf("🚀 Spawning MailHog (SMTP:%d  UI:%d)...\n", smtpPort, uiPort)

	runArgs := []string{
		"run", "-d",
		"--name", mailContainerName,
		"-p", fmt.Sprintf("%d:1025", smtpPort), // SMTP
		"-p", fmt.Sprintf("%d:8025", uiPort),   // HTTP UI
		"--label", "managed-by=devx",
		"--label", "devx-mail=mailhog",
		"--restart", "unless-stopped",
		mailImage,
	}

	out, err := exec.Command(runtime, runArgs...).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "address already in use") ||
			strings.Contains(string(out), "port is already allocated") {
			return fmt.Errorf("port %d or %d is already in use — choose different ports with --smtp-port / --ui-port", smtpPort, uiPort)
		}
		return fmt.Errorf("failed to start MailHog: %w\nOutput: %s", err, string(out))
	}

	fmt.Printf("\n%s MailHog is running!\n\n", tui.IconDone)
	printMailEnvVars(smtpPort, uiPort)
	fmt.Printf("\n  Stop:   devx mail rm\n")
	return nil
}

func printMailEnvVars(smtpPort, uiPort int) {
	uiURL := fmt.Sprintf("http://localhost:%d", uiPort)
	apiURL := fmt.Sprintf("http://localhost:%d/api/v2/messages", uiPort)

	if outputJSON {
		type mailJSON struct {
			SMTPHost       string `json:"smtp_host"`
			SMTPPort       int    `json:"smtp_port"`
			UIUrl          string `json:"ui_url"`
			APIUrl         string `json:"api_url"`
		}
		b, _ := json.MarshalIndent(mailJSON{
			SMTPHost: "localhost",
			SMTPPort: smtpPort,
			UIUrl:    uiURL,
			APIUrl:   apiURL,
		}, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("  Add to your .env:")
	fmt.Println()
	fmt.Printf("    SMTP_HOST=localhost\n")
	fmt.Printf("    SMTP_PORT=%d\n", smtpPort)
	fmt.Printf("    MAIL_CATCHER_URL=%s\n", uiURL)
	fmt.Println()
	fmt.Printf("  Web UI:  %s\n", tui.StyleURL.Render(uiURL))
	fmt.Printf("  API:     %s\n", tui.StyleURL.Render(apiURL))
	fmt.Println()
	fmt.Println("  Or use 'devx shell' to have these injected automatically.")
}

// discoverMailEnvVars checks if the mail catcher container is running and
// returns its env vars for injection into devx shell.
func discoverMailEnvVars(runtime string) map[string]string {
	out, err := exec.Command(runtime, "inspect", mailContainerName,
		"--format", "{{.State.Running}}\t{{.HostConfig.PortBindings}}").Output()
	if err != nil {
		return nil
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	if len(parts) < 1 || parts[0] != "true" {
		return nil
	}

	// Parse the actual bound port for SMTP from the running container.
	// Falls back to the default if we can't parse the docker inspect output.
	smtpPort := mailSMTPPort
	uiPort := mailUIPort

	// Try to extract the actual host port using a separate query
	smtpPortOut, err := exec.Command(runtime, "inspect", mailContainerName,
		"--format", "{{(index .HostConfig.PortBindings \"1025/tcp\" 0).HostPort}}").Output()
	if err == nil {
		if p := strings.TrimSpace(string(smtpPortOut)); p != "" {
			fmt.Sscanf(p, "%d", &smtpPort) //nolint:errcheck
		}
	}
	uiPortOut, err := exec.Command(runtime, "inspect", mailContainerName,
		"--format", "{{(index .HostConfig.PortBindings \"8025/tcp\" 0).HostPort}}").Output()
	if err == nil {
		if p := strings.TrimSpace(string(uiPortOut)); p != "" {
			fmt.Sscanf(p, "%d", &uiPort) //nolint:errcheck
		}
	}

	return map[string]string{
		"SMTP_HOST":        "localhost",
		"SMTP_PORT":        fmt.Sprintf("%d", smtpPort),
		"MAIL_CATCHER_URL": fmt.Sprintf("http://localhost:%d", uiPort),
	}
}
