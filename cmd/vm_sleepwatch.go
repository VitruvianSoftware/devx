package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/spf13/cobra"
)

var sleepTimeout int

var vmDaemonCmd = &cobra.Command{
	Use:   "sleep-watch",
	Short: "Run a background daemon to auto-sleep idle VMs",
	Long:  `Runs continuously, pausing the local VM backend when no containers are actively running to save battery and RAM.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, err := getVMProvider()
		if err != nil {
			return err
		}

		devName := os.Getenv("USER")
		if devName == "" {
			devName = "developer"
		}
		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil && s.DevHostname != "" {
			cfg.DevHostname = s.DevHostname
		}
		if cfg.DevHostname == "" {
			cfg.DevHostname = "devx"
		}

		// Determine the container list command based on the provider.
		// Podman and Docker use their native CLI; Lima/Colima use nerdctl via SSH.
		psCommand := containerPSCommand(prov.Name(), cfg.DevHostname)

		fmt.Printf("🐾 devx sleep-watch active (provider: %s). Polling every %d seconds...\n", prov.Name(), sleepTimeout)

		idleTicks := 0

		for {
			time.Sleep(time.Duration(sleepTimeout) * time.Second)

			if !prov.IsRunning(cfg.DevHostname) {
				continue // already asleep
			}

			// Count running containers using the provider-appropriate command
			var out bytes.Buffer
			psCmd := exec.Command(psCommand[0], psCommand[1:]...)
			psCmd.Stdout = &out

			if err := psCmd.Run(); err != nil {
				continue
			}

			running := len(strings.Fields(strings.TrimSpace(out.String())))

			if running > 0 {
				idleTicks = 0
			} else {
				idleTicks++
			}

			// If idle for 3 consecutive polls, sleep it!
			if idleTicks >= 3 {
				fmt.Println("💤 No active containers inside VM. Triggering deep-sleep...")
				if err := prov.Sleep(cfg.DevHostname); err == nil {
					fmt.Println("✓ VM is asleep.")
				}
				idleTicks = 0
			}
		}
	},
}

// containerPSCommand returns the CLI command to list running container IDs
// for the given provider.
func containerPSCommand(providerName, devHostname string) []string {
	switch providerName {
	case "docker":
		return []string{"docker", "ps", "-q"}
	case "lima":
		return []string{"limactl", "shell", devHostname, "nerdctl", "ps", "-q"}
	case "colima":
		return []string{"colima", "ssh", "--", "nerdctl", "ps", "-q"}
	default:
		// podman is the default
		return []string{"podman", "ps", "-q"}
	}
}

func init() {
	vmDaemonCmd.Flags().IntVarP(&sleepTimeout, "interval", "i", 60, "Interval in seconds to poll container states")
	vmCmd.AddCommand(vmDaemonCmd)
}

