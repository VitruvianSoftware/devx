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
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/usb"
)

func newClusterUSBCmd(configFile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usb",
		Short: "Build a Ventoy USB that boots a laptop into a self-joining node",
		Long: `usb generates the artifacts for a Ventoy multi-boot USB stick. Boot a
supported x86_64 laptop off it, pick a menu entry, and it self-provisions a K3s
node that joins this cluster — over Tailscale with a LAN fallback, as an agent.

Two lifecycle modes are produced per renderer: an ephemeral live-boot (runs from
RAM; the internal disk is untouched) and a persistent install (writes the OS to
disk). Three renderers package the same join recipe differently: fcos
(Ignition), ubuntu (cloud-init), and baked (prebuilt image + config partition).

devx generates and stages the provisioning payloads + ventoy.json; copying the
base ISOs and writing to a physical stick are operator/field steps (see the
design spec for the hardware checklist).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newClusterUSBBuildCmd(configFile),
		newClusterUSBPruneCmd(configFile),
	)
	return cmd
}

func newClusterUSBBuildCmd(configFile *string) *cobra.Command {
	var (
		outputDir string
		renderers []string
		modeFlags []string
		role      string
		timeout   time.Duration
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Generate boot artifacts and ventoy.json into an output directory",
		Long: `build resolves this cluster's join coordinates (server URL + token, reusing
the same healthy-server discovery as 'cluster join'), then renders the selected
renderers across the selected modes into --output, alongside a ventoy.json and a
MANIFEST.md. Use --dry-run to print the boot-menu plan without touching the
cluster or writing files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			modes, err := parseModes(modeFlags)
			if err != nil {
				return err
			}
			if role != "" && role != string(usb.RoleAgent) && role != string(usb.RoleServer) {
				return fmt.Errorf("invalid --role %q (want agent or server)", role)
			}

			ctx := contextWithSignal(cmd.Context(), timeout)
			res, err := usb.Build(ctx, cfg, usb.BuildOptions{
				OutputDir: outputDir,
				Renderers: renderers,
				Modes:     modes,
				Role:      usb.Role(role),
				DryRun:    DryRun,
			})
			if err != nil {
				return err
			}
			return printUSBBuildResult(res)
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "devx-usb", "directory to stage USB artifacts into")
	cmd.Flags().StringSliceVar(&renderers, "renderer", nil, "renderers to build (fcos,ubuntu,baked); default all/config")
	cmd.Flags().StringSliceVar(&modeFlags, "mode", nil, "lifecycle modes (ephemeral,install); default both")
	cmd.Flags().StringVar(&role, "role", "agent", "k3s role for install entries (agent or server)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "maximum time to resolve cluster coordinates")
	return cmd
}

func newClusterUSBPruneCmd(configFile *string) *cobra.Command {
	var (
		ttl     time.Duration
		timeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove NotReady ephemeral USB nodes from the cluster (ghost reaper)",
		Long: `prune drains and deletes nodes that joined via an ephemeral live-boot and
have since gone NotReady (e.g. the laptop was unplugged) for longer than --ttl.
It only touches nodes carrying the devx.io/ephemeral label, so permanent nodes
are never reaped. Use --dry-run to see what would be removed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			// Config default for TTL when the flag was not explicitly set.
			if !cmd.Flags().Changed("ttl") && cfg.Cluster.USB.NodeTTL != "" {
				d, err := time.ParseDuration(cfg.Cluster.USB.NodeTTL)
				if err != nil {
					return fmt.Errorf("parsing cluster.usb.nodeTTL %q: %w", cfg.Cluster.USB.NodeTTL, err)
				}
				ttl = d
			}

			ctx := contextWithSignal(cmd.Context(), timeout)
			res, err := usb.Prune(ctx, cfg, usb.PruneOptions{TTL: ttl, DryRun: DryRun})
			if err != nil {
				return err
			}
			return printUSBPruneResult(res)
		},
	}

	cmd.Flags().DurationVar(&ttl, "ttl", 30*time.Minute, "remove ephemeral nodes NotReady longer than this")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "maximum time for the operation")
	return cmd
}

// parseModes converts --mode flags into usb.Mode values.
func parseModes(in []string) ([]usb.Mode, error) {
	var out []usb.Mode
	for _, m := range in {
		switch usb.Mode(m) {
		case usb.ModeEphemeral:
			out = append(out, usb.ModeEphemeral)
		case usb.ModeInstall:
			out = append(out, usb.ModeInstall)
		default:
			return nil, fmt.Errorf("invalid --mode %q (want ephemeral or install)", m)
		}
	}
	return out, nil
}

func printUSBBuildResult(res *usb.BuildResult) error {
	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(res)
	}
	if res.DryRun {
		fmt.Println("📋 Dry-run: USB boot menu plan (no files written, cluster not contacted)")
	} else {
		fmt.Printf("✅ Staged USB artifacts in %s\n", res.OutputDir)
	}
	fmt.Println()
	for _, e := range res.Entries {
		fmt.Printf("  • %-44s [%s/%s, %s]\n", e.MenuTitle, e.Renderer, e.Mode, e.Injection)
	}
	fmt.Println("\n  Base images to place on the stick:")
	for _, img := range res.RequiredImages {
		fmt.Printf("    - %s\n", img)
	}
	if !res.DryRun {
		fmt.Printf("\n  See %s/MANIFEST.md and %s/ventoy/ventoy.json\n", res.OutputDir, res.OutputDir)
	}
	return nil
}

func printUSBPruneResult(res *usb.PruneResult) error {
	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(res)
	}
	if len(res.Stale) == 0 {
		fmt.Println("✅ No stale ephemeral nodes to prune.")
		return nil
	}
	if res.DryRun {
		fmt.Println("📋 Dry-run: would remove stale ephemeral nodes:")
		for _, n := range res.Stale {
			fmt.Printf("  - %s\n", n)
		}
		return nil
	}
	fmt.Printf("🗑️  Removed %d stale ephemeral node(s):\n", len(res.Removed))
	for _, n := range res.Removed {
		fmt.Printf("  - %s\n", n)
	}
	return nil
}
