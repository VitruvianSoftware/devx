package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/provider"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/spf13/cobra"
)

var sleepTimeout int

var vmDaemonCmd = &cobra.Command{
	Use:   "sleep-watch",
	Short: "Run a background daemon to auto-sleep idle VMs",
	Long:  `Runs continuously, pausing the local VM backend (e.g. Podman) when no containers are actively running to save battery and RAM.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, err := provider.Get("podman")
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

		fmt.Printf("🐾 devx sleep-watch active. Polling every %d seconds...\n", sleepTimeout)

		idleTicks := 0

		for {
			time.Sleep(time.Duration(sleepTimeout) * time.Second)

			if !prov.IsRunning(cfg.DevHostname) {
				continue // already asleep
			}

			// We need to count containers. If podman ps is empty, we increment idle ticks.
			var out bytes.Buffer
			psCmd := exec.Command("podman", "ps", "-q")
			psCmd.Stdout = &out

			// If error, maybe podman machine is booting or failing, just skip this tick
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

func init() {
	vmDaemonCmd.Flags().IntVarP(&sleepTimeout, "interval", "i", 60, "Interval in seconds to poll container states")
	vmCmd.AddCommand(vmDaemonCmd)
}
