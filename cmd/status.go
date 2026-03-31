package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/podman"
	"github.com/VitruvianSoftware/devx/internal/tailscale"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the health of the VM, Cloudflare tunnel, and Tailscale",
	RunE:  runStatus,
}

func runStatus(_ *cobra.Command, _ []string) error {
	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, "", "", "")

	fmt.Print(tui.StyleTitle.Render("devx — Status") + "\n\n")

	// VM
	vmStatus := "not created"
	vmStyle := tui.StyleMuted
	
	if info, err := podman.Inspect(cfg.DevHostname); err == nil {
		vmStatus = info.State
		if vmStatus == "" {
			vmStatus = "stopped"
		}
		if vmStatus == "running" {
			vmStyle = tui.StyleDetailDone
		} else {
			vmStyle = tui.StyleDetailError
		}
	}
	printStatusRow("VM", cfg.DevHostname, vmStyle.Render(vmStatus))

	// Cloudflare tunnel
	cfStatus, err := cloudflare.TunnelStatus(cfg.TunnelName)
	if err != nil {
		cfStatus = "error: " + err.Error()
	}
	cfStyle := tui.StyleDetailDone
	if err != nil {
		cfStyle = tui.StyleDetailError
	}
	printStatusRow("Cloudflare", cfg.CFDomain, cfStyle.Render(cfStatus))

	// Tailscale (only if VM is running)
	tsStatus := "vm not running"
	tsStyle := tui.StyleMuted
	if podman.IsRunning(cfg.DevHostname) {
		tsStatus = tailscale.Status(cfg.DevHostname)
		tsStyle = tui.StyleDetailDone
	}
	printStatusRow("Tailscale", cfg.DevHostname, tsStyle.Render(tsStatus))

	fmt.Println()
	fmt.Println(tui.StyleMuted.Render("  Public endpoint:  https://" + cfg.CFDomain + " → :8080"))
	fmt.Println()

	if tunnels, err := cloudflare.ListExposedTunnels(devName); err == nil && len(tunnels) > 0 {
		fmt.Println("  " + tui.StyleTitle.Render("Exposed Ports") + "\n")
		prefix := fmt.Sprintf("devx-expose-%s-", devName)
		for _, t := range tunnels {
			exposeID := strings.TrimPrefix(t.Name, prefix)
			fullDomain := fmt.Sprintf("%s.%s", exposeID, cfg.CFDomain)
			fmt.Printf("    https://%-30s %s\n", fullDomain, tui.StyleMuted.Render("("+t.Name+")"))
		}
		fmt.Println()
	}

	return nil
}

func printStatusRow(label, name, status string) {
	fmt.Printf("  %s  %s  %s\n",
		tui.StyleLabel.Render(label),
		tui.StyleStepName.Render(name),
		status,
	)
}

func init() {
	vmCmd.AddCommand(statusCmd)
}
