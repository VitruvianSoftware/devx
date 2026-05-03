// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/provider"
)

var vmProviderFlag string

var vmCmd = &cobra.Command{
	Use:   "vm",
	GroupID: "infra",
	Short: "Manage the local development VM",
	Long: `Commands for provisioning, inspecting, and managing your local dev environment.
Supports multiple backends via --provider (podman, docker, orbstack, lima, colima).
Use --provider=auto to auto-detect, or set a default in ~/.devx/config.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// getVMProvider resolves the provider using a three-tier cascade:
//  1. --provider flag (explicit CLI override)
//  2. ~/.devx/config.yaml provider field (machine-local)
//  3. devx.yaml provider field (project-level)
//  4. "auto" (detect installed backends, prompt if ambiguous)
func getVMProvider() (provider.VMProvider, error) {
	name := resolveProviderName()

	vm, _, err := provider.Resolve(name)
	if err != nil {
		// If multiple providers are detected, show an interactive picker
		var multiErr *provider.MultipleProvidersError
		if errors.As(err, &multiErr) {
			return pickProviderInteractively(multiErr.Available)
		}
		return nil, err
	}
	return vm, nil
}

// getFullProvider returns the composite Provider (VM + Runtime).
// Use this when the command needs container runtime access (e.g. shell, checkpoint).
func getFullProvider() (*provider.Provider, error) {
	name := resolveProviderName()

	vm, rt, err := provider.Resolve(name)
	if err != nil {
		var multiErr *provider.MultipleProvidersError
		if errors.As(err, &multiErr) {
			vm, err := pickProviderInteractively(multiErr.Available)
			if err != nil {
				return nil, err
			}
			// Re-resolve to get the runtime
			_, rt, _ = provider.Resolve(vm.Name())
			return &provider.Provider{VM: vm, Runtime: rt}, nil
		}
		return nil, err
	}
	return &provider.Provider{VM: vm, Runtime: rt}, nil
}

// resolveProviderName walks the three-tier cascade and returns the resolved
// provider name string ("podman", "lima", "auto", etc.).
func resolveProviderName() string {
	// Tier 1: CLI flag
	if vmProviderFlag != "" {
		return vmProviderFlag
	}

	// Tier 2: Machine-local config (~/.devx/config.yaml)
	if local, err := config.LoadLocal(); err == nil && local.Provider != "" {
		return local.Provider
	}

	// Tier 3: Project config (devx.yaml)
	if p := readProjectProvider(); p != "" {
		return p
	}

	// Tier 4: Auto-detect
	return "auto"
}

// readProjectProvider reads the provider field from devx.yaml in the current directory.
func readProjectProvider() string {
	yamlPath, err := findDevxConfig()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return ""
	}
	var cfg struct {
		Provider string `yaml:"provider"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.Provider
}

// pickProviderInteractively shows a huh select prompt when multiple providers
// are detected and no preference is configured.
func pickProviderInteractively(available []provider.DetectedProvider) (provider.VMProvider, error) {
	if NonInteractive {
		names := make([]string, len(available))
		for i, d := range available {
			names[i] = d.Name
		}
		return nil, fmt.Errorf("multiple VM backends detected (%v) — set --provider or configure in ~/.devx/config.yaml", names)
	}

	options := make([]huh.Option[string], len(available))
	for i, d := range available {
		label := d.Name
		if d.Version != "" {
			label = fmt.Sprintf("%s (%s)", d.Name, d.Version)
		}
		options[i] = huh.NewOption(label, d.Name)
	}

	var selected string
	err := huh.NewSelect[string]().
		Title("Multiple VM backends detected. Which would you like to use?").
		Options(options...).
		Value(&selected).
		Run()
	if err != nil {
		return nil, fmt.Errorf("provider selection cancelled: %w", err)
	}

	// Offer to save the choice
	var save bool
	_ = huh.NewConfirm().
		Title(fmt.Sprintf("Save %q as your default provider in ~/.devx/config.yaml?", selected)).
		Value(&save).
		Run()

	if save {
		_ = config.SaveLocal(&config.LocalConfig{Provider: selected})
		fmt.Printf("✓ Saved provider=%s to %s\n", selected, config.LocalConfigPath())
	}

	return provider.Get(selected)
}

func init() {
	vmCmd.PersistentFlags().StringVar(&vmProviderFlag, "provider", "",
		"Virtualization backend to use (podman, docker, orbstack, lima, colima, auto)")
}

