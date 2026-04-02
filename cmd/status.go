package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/tailscale"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the health of the VM, Cloudflare tunnel, and Tailscale",
	RunE:  runStatus,
}

func runStatus(_ *cobra.Command, _ []string) error {
	vm, err := getVMProvider()
	if err != nil {
		return err
	}

	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, "", "", "")

	if !outputJSON {
		fmt.Print(tui.StyleTitle.Render("devx — Status") + "\n\n")
	}

	// VM
	vmStatus := "not created"
	vmStyle := tui.StyleMuted

	if info, err := vm.Inspect(cfg.DevHostname); err == nil {
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
	if !outputJSON {
		printStatusRow("VM", cfg.DevHostname+" ("+vm.Name()+")", vmStyle.Render(vmStatus))
	}

	// Cloudflare tunnel
	cfStatus, err := cloudflare.TunnelStatus(cfg.TunnelName)
	if err != nil {
		cfStatus = "error: " + err.Error()
	}
	cfStyle := tui.StyleDetailDone
	if err != nil {
		cfStyle = tui.StyleDetailError
	}
	if !outputJSON {
		printStatusRow("Cloudflare", cfg.CFDomain, cfStyle.Render(cfStatus))
	}

	// Tailscale (only if VM is running)
	tsStatus := "vm not running"
	tsStyle := tui.StyleMuted
	if vm.IsRunning(cfg.DevHostname) {
		sshFn := func(machine, command string) (string, error) {
			return vm.SSH(machine, command)
		}
		tsStatus = tailscale.StatusWithSSH(cfg.DevHostname, sshFn)
		tsStyle = tui.StyleDetailDone
	}
	if !outputJSON {
		printStatusRow("Tailscale", cfg.DevHostname, tsStyle.Render(tsStatus))

		fmt.Println()
		fmt.Println(tui.StyleMuted.Render("  Public endpoint:  https://" + cfg.CFDomain + " → :8080"))
		fmt.Println()
	}

	if tunnels, err := cloudflare.ListExposedTunnels(devName); err == nil && len(tunnels) > 0 {
		if !outputJSON {
			fmt.Println("  " + tui.StyleTitle.Render("Exposed Ports") + "\n")
		}
		prefix := fmt.Sprintf("devx-expose-%s-", devName)
		for _, t := range tunnels {
			exposeID := strings.TrimPrefix(t.Name, prefix)
			fullDomain := exposure.GenerateDomain(exposeID, cfg.CFDomain)
			if !outputJSON {
				fmt.Printf("    https://%-30s %s\n", fullDomain, tui.StyleMuted.Render("("+t.Name+")"))
			}
		}
		if !outputJSON {
			fmt.Println()
		}
	}

	if outputJSON {
		type exposedPort struct {
			Name   string `json:"name"`
			Domain string `json:"domain"`
		}

		var ports []exposedPort
		if tunnels, err := cloudflare.ListExposedTunnels(devName); err == nil {
			prefix := fmt.Sprintf("devx-expose-%s-", devName)
			for _, t := range tunnels {
				exposeID := strings.TrimPrefix(t.Name, prefix)
				ports = append(ports, exposedPort{
					Name:   t.Name,
					Domain: exposure.GenerateDomain(exposeID, cfg.CFDomain),
				})
			}
		}

		outJSON := struct {
			VM struct {
				Name     string `json:"name"`
				Provider string `json:"provider"`
				State    string `json:"state"`
			} `json:"vm"`
			Cloudflare struct {
				Domain string `json:"domain"`
				Status string `json:"status"`
			} `json:"cloudflare"`
			Tailscale struct {
				Status string `json:"status"`
			} `json:"tailscale"`
			ExposedPorts []exposedPort `json:"exposed_ports"`
		}{
			ExposedPorts: ports,
		}

		outJSON.VM.Name = cfg.DevHostname
		outJSON.VM.Provider = vm.Name()
		outJSON.VM.State = vmStatus
		outJSON.Cloudflare.Domain = cfg.CFDomain
		outJSON.Cloudflare.Status = cfStatus
		outJSON.Tailscale.Status = tsStatus

		enc, _ := json.MarshalIndent(outJSON, "", "  ")
		fmt.Println(string(enc))
		return nil
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
