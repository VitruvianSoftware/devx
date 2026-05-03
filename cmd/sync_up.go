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
	"path/filepath"
	"strings"

	devxsync "github.com/VitruvianSoftware/devx/internal/sync"
	"github.com/spf13/cobra"
)

var syncUpRuntime string

var syncUpCmd = &cobra.Command{
	Use:   "up [names...]",
	Short: "Start file sync sessions defined in devx.yaml",
	Long: `Create Mutagen sync sessions for all services with 'sync:' blocks in devx.yaml.
If one or more service names are provided, only those services are synced.

Sync sessions bypass slow VirtioFS volume mounts by injecting file changes
directly into running containers in milliseconds.

Examples:
  devx sync up           # start all sync sessions from devx.yaml
  devx sync up api       # start only the api service sync
  devx sync up --dry-run # preview sync commands without executing`,
	RunE: runSyncUp,
}

func init() {
	syncUpCmd.Flags().StringVar(&syncUpRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	syncCmd.AddCommand(syncUpCmd)
}

func runSyncUp(_ *cobra.Command, args []string) error {
	if !devxsync.IsInstalled() {
		return fmt.Errorf("mutagen is not installed. Run: devx doctor install --all")
	}

	// Idea 44: use the unified config resolver so include blocks are processed
	yamlPath, err := mustFindDevxConfig()
	if err != nil {
		return err
	}
	cfg, err := resolveConfig(yamlPath, "")
	if err != nil {
		return fmt.Errorf("could not read devx.yaml: %w", err)
	}

	// Collect all sync mappings
	type syncTarget struct {
		ServiceName string
		Container   string
		Src         string
		Dest        string
		Ignore      []string
	}

	var targets []syncTarget
	wantSet := make(map[string]bool, len(args))
	for _, a := range args {
		wantSet[strings.ToLower(a)] = true
	}

	cwd, _ := os.Getwd()
	for _, svc := range cfg.Services {
		if len(svc.Sync) == 0 {
			continue
		}
		if len(wantSet) > 0 && !wantSet[strings.ToLower(svc.Name)] {
			continue
		}
		// Idea 44: use svc.Dir for services from included projects, fallback to cwd
		baseDir := svc.Dir
		if baseDir == "" {
			baseDir = cwd
		}
		for _, s := range svc.Sync {
			src := s.Src
			if !filepath.IsAbs(src) {
				src = filepath.Join(baseDir, src)
			}
			targets = append(targets, syncTarget{
				ServiceName: svc.Name,
				Container:   s.Container,
				Src:         src,
				Dest:        s.Dest,
				Ignore:      s.Ignore,
			})
		}
	}

	if len(targets) == 0 {
		if len(wantSet) > 0 {
			return fmt.Errorf("no sync mappings found for services: %v. Add 'sync:' blocks to your services in devx.yaml", args)
		}
		fmt.Println("No sync mappings found in devx.yaml. Add 'sync:' blocks to your services.")
		return nil
	}

	if DryRun {
		for _, t := range targets {
			allIgnores := append(devxsync.DefaultIgnores(), t.Ignore...)
			ignoreFlags := ""
			for _, ig := range allIgnores {
				ignoreFlags += fmt.Sprintf(" --ignore %s", ig)
			}
			fmt.Printf("[dry-run] would execute: mutagen sync create --name devx-sync-%s --label managed-by=devx%s %s docker://%s%s\n",
				t.ServiceName, ignoreFlags, t.Src, t.Container, t.Dest)
		}
		return nil
	}

	// Ensure Mutagen daemon is running
	if err := devxsync.EnsureDaemon(); err != nil {
		return fmt.Errorf("failed to start mutagen daemon: %w", err)
	}

	type sessionResult struct {
		Name      string `json:"name"`
		Src       string `json:"src"`
		Dest      string `json:"dest"`
		Container string `json:"container"`
		Status    string `json:"status"`
	}
	var results []sessionResult

	for _, t := range targets {
		fmt.Printf("⚡ Creating sync session: %s → docker://%s%s\n", t.Src, t.Container, t.Dest)

		if err := devxsync.CreateSession(t.ServiceName, t.Src, t.Dest, t.Container, syncUpRuntime, t.Ignore); err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "unable to connect") || strings.Contains(errMsg, "No such container") {
				return fmt.Errorf("container %q is not running. Start it first with 'devx up' or manually", t.Container)
			}
			return fmt.Errorf("failed to create sync session for %s: %w", t.ServiceName, err)
		}

		results = append(results, sessionResult{
			Name:      "devx-sync-" + t.ServiceName,
			Src:       t.Src,
			Dest:      fmt.Sprintf("docker://%s%s", t.Container, t.Dest),
			Container: t.Container,
			Status:    "created",
		})

		fmt.Printf("  ✅ devx-sync-%s is active\n", t.ServiceName)
	}

	if outputJSON {
		enc, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	fmt.Printf("\n🎉 %d sync session(s) active. File changes will propagate in milliseconds.\n", len(results))
	fmt.Println("   Run 'devx sync list' to monitor sessions.")
	fmt.Println("   Run 'devx sync rm' to stop all sync sessions.")
	return nil
}
