package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/devxerr"
)

// ensureVMRunning is a lifecycle hook invoked by devx CLI commands
// that require the local hypervisor/provider to be un-suspended.
func ensureVMRunning() error {
	if DryRun {
		return nil
	}

	prov, err := getVMProvider()
	if err != nil {
		return err
	}

	// Orbital/Docker Desktop manages sleep state natively.
	if prov.Name() != "podman" {
		return nil
	}

	if !prov.IsRunning("devx") {
		fmt.Fprintln(os.Stderr, "😴 VM is asleep. Waking it up...")
		if err := prov.Start("devx"); err != nil {
			if strings.Contains(err.Error(), "VM does not exist") {
				return devxerr.New(devxerr.CodeVMNotFound, "VM does not exist. Run 'devx init' to create it.", err)
			}
			return devxerr.New(devxerr.CodeVMDormant, "Failed to wake up VM", err)
		}
	}

	return nil
}
