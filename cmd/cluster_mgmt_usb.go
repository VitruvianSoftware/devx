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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/usb"
)

func newClusterUSBCmd(configFile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usb",
		Short: "Build a USB that boots a laptop into a self-joining node",
		Long: `usb generates the artifacts for a self-joining cluster-node USB stick. Boot a
supported x86_64 laptop off it and it self-provisions a K3s node that joins this
cluster — over Tailscale with a LAN fallback, as an agent.

Two lifecycle modes are produced per renderer: an ephemeral live-boot (runs from
RAM; the internal disk is untouched) and a persistent install (writes the OS to
disk). Two renderers package the same join recipe differently: fcos (a native GPT
Fedora CoreOS disk built by coreos-installer with the Ignition embedded) and
baked (prebuilt image + config partition).

devx generates and stages the provisioning payloads; with --assemble/--device it
also builds (and optionally flashes) a bootable image in a Lima builder VM (see
the design spec for the hardware checklist).`,
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
		device    string
		assemble  bool
		imageSize string
		imageOut  string
		assumeYes bool
		builderVM string
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Generate boot artifacts, or assemble + flash a bootable FCOS USB",
		Long: `build resolves this cluster's join coordinates (reusing the same healthy-server
discovery as 'cluster join') and renders the boot payloads (Ignition, join script)
into --output.

With --device or --assemble it goes further: it spins up a Lima builder VM and
runs coreos-installer to write a native GPT Fedora CoreOS disk image — the metal
image is fetched from the FCOS stream and the compiled Ignition is embedded —
producing a flashable .img. With --device it then flashes that image to the
(removable-only) device after confirmation. Assembled sticks are ephemeral-only,
so booting one can never format a host laptop's internal disk. FCOS grows its
root partition to fill the real USB on first boot.

The builder VM (--builder-vm) is kept between runs; remove it with
'limactl delete <name>' when you no longer need it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if role != "" && role != string(usb.RoleAgent) && role != string(usb.RoleServer) {
				return fmt.Errorf("invalid --role %q (want agent or server)", role)
			}
			assembling := device != "" || assemble

			modes, err := parseModes(modeFlags)
			if err != nil {
				return err
			}
			buildRenderers, buildModes := renderers, modes
			if assembling {
				// Assembled sticks are FCOS, ephemeral-only (host-disk-safe): we only
				// need the FCOS Butane the assembly script compiles + embeds.
				buildRenderers = []string{"fcos"}
				buildModes = []usb.Mode{usb.ModeEphemeral}
			}

			ctx := contextWithSignal(cmd.Context(), timeout)
			res, err := usb.Build(ctx, cfg, usb.BuildOptions{
				OutputDir: outputDir,
				Renderers: buildRenderers,
				Modes:     buildModes,
				Role:      usb.Role(role),
				DryRun:    DryRun,
			})
			if err != nil {
				return err
			}
			if !assembling {
				return printUSBBuildResult(res)
			}
			return assembleAndFlash(ctx, assembleFlags{
				outputDir: outputDir, device: device, imageSize: imageSize,
				imageOut: imageOut, assumeYes: assumeYes, builderVM: builderVM,
			})
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "devx-usb", "directory to stage USB artifacts into")
	cmd.Flags().StringSliceVar(&renderers, "renderer", nil, "renderers to build (fcos,baked); default all/config")
	cmd.Flags().StringSliceVar(&modeFlags, "mode", nil, "lifecycle modes (ephemeral,install); default both")
	cmd.Flags().StringVar(&role, "role", "agent", "k3s role for install entries (agent or server)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "maximum time for the operation")
	cmd.Flags().StringVar(&device, "device", "", "flash the assembled image to this removable device (e.g. /dev/disk4)")
	cmd.Flags().BoolVar(&assemble, "assemble", false, "assemble a bootable image via a Lima builder VM (implied by --device)")
	cmd.Flags().StringVar(&imageSize, "image-size", "", "sparse image size for the FCOS metal layout (default 4G; FCOS grows root to fill the stick on first boot)")
	cmd.Flags().StringVar(&imageOut, "image-out", "devx-usb.img", "where to write the assembled image when --device is not set")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the destructive-write confirmation")
	cmd.Flags().StringVar(&builderVM, "builder-vm", "devx-usb-builder", "name of the Lima builder VM (kept between runs)")
	return cmd
}

type assembleFlags struct {
	outputDir, device, imageSize, imageOut, builderVM string
	assumeYes                                         bool
}

// assembleAndFlash builds the GPT Fedora CoreOS image in a Lima VM and, when a
// device is given, flashes it. Staging into outputDir has already happened. The
// image is sized only for the FCOS metal layout; FCOS grows root to fill the real
// USB on first boot, so the image size is independent of the target device size.
func assembleAndFlash(ctx context.Context, f assembleFlags) error {
	sizeMB := usb.DefaultImageSizeMB
	if f.imageSize != "" {
		var err error
		if sizeMB, err = usb.ParseSizeMB(f.imageSize); err != nil {
			return err
		}
	}

	if DryRun {
		fmt.Printf("📋 Dry-run: would assemble a %dM GPT Fedora CoreOS image (coreos-installer) in VM %q\n",
			sizeMB, f.builderVM)
		if f.device != "" {
			fmt.Printf("           then flash to %s (after confirmation)\n", f.device)
		}
		return nil
	}

	params := usb.AssemblyParams{
		ImageSizeMB: sizeMB,
		ButaneVM:    "/var/tmp/devx/payload/fcos/ephemeral/devx-join.bu",
		ImageVM:     "/var/tmp/devx/devx-usb.img",
	}
	b := usb.NewBuilder(f.builderVM)
	fmt.Printf("🔧 Provisioning builder VM %q...\n", f.builderVM)
	if err := b.EnsureBuilderVM(ctx); err != nil {
		return err
	}
	fmt.Println("📦 Assembling GPT Fedora CoreOS image in the VM (compile Ignition + coreos-installer)...")
	img, err := b.BuildImage(ctx, params, f.outputDir)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(img) }() // multi-GB temp; remove after flash/move

	if f.device == "" {
		if err := moveFile(img, f.imageOut); err != nil {
			return err
		}
		fmt.Printf("✅ Assembled image written to %s\n", f.imageOut)
		return nil
	}
	confirm := func() bool {
		if f.assumeYes {
			return true
		}
		fmt.Printf("⚠️  About to ERASE %s and flash the Fedora CoreOS image. Type 'yes' to continue: ", f.device)
		var s string
		_, _ = fmt.Scanln(&s)
		return s == "yes"
	}
	if err := usb.Flash(ctx, img, f.device, confirm); err != nil {
		return err
	}
	fmt.Printf("✅ Flashed bootable Fedora CoreOS stick to %s\n", f.device)
	return nil
}

// moveFile moves src to dst, trying a fast rename first and falling back to a
// copy when src and dst are on different filesystems (the common temp→cwd case).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst) // don't leave a partial image behind
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
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
	fmt.Println("\n  Boot media:")
	for _, img := range res.RequiredImages {
		fmt.Printf("    - %s\n", img)
	}
	if !res.DryRun {
		fmt.Printf("\n  See %s/MANIFEST.md\n", res.OutputDir)
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
