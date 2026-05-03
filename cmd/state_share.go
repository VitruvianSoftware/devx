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
	"time"

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/state"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var stateShareDbOnly bool
var stateShareRelay string

var stateShareCmd = &cobra.Command{
	Use:   "share",
	Short: "Bundle and share your running environment state with a teammate",
	Long: `Bundles your running container states, database volume snapshots,
and environment metadata into a single encrypted, portable artifact.

By default, this captures CRIU checkpoints of your running containers (requires Podman)
and exports your database volumes. The bundle is encrypted locally using AES-256-GCM.
You must configure a relay destination (s3:// or gs://) in devx.yaml or pass --relay.`,
	Example: `  # Share full state using the relay defined in devx.yaml
  devx state share
  
  # Override relay destination
  devx state share --relay s3://my-team-bucket/state
  
  # Share database volumes only (useful for non-Podman users)
  devx state share --db-only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, err := getFullProvider()
		if err != nil {
			return err
		}

		rt := prov.Runtime

		// Force db-only if not Podman
		if rt.Name() != "podman" && !stateShareDbOnly {
			if !outputJSON {
				fmt.Println("⚠️  Warning: CRIU checkpoints require Podman. Auto-enabling --db-only mode.")
			}
			stateShareDbOnly = true
		}

		// Load config to find databases
		yamlPath, err := findDevxConfig()
		var cfg *DevxConfig
		if err == nil {
			cfg, _ = resolveConfig(yamlPath, "")
		}

		// Resolve relay
		relayCfg := stateShareRelay
		if relayCfg == "" && cfg != nil && cfg.State != nil {
			relayCfg = cfg.State.Relay
		}

		if !DryRun {
			if relayCfg == "" {
				return &devxerr.DevxError{
					ExitCode: devxerr.CodeStateShareUploadFailed,
					Message:  "No relay configured. Set state.relay in devx.yaml or pass --relay s3://...",
				}
			}
			if _, _, err := state.ParseRelay(relayCfg); err != nil {
				return &devxerr.DevxError{
					ExitCode: devxerr.CodeStateShareUploadFailed,
					Message:  err.Error(),
					Err:      err,
				}
			}
		}

		bundleID := uuid.New().String()
		checkpointName := fmt.Sprintf("_share_%d", time.Now().Unix())

		// Cleanup temps
		defer func() {
			if !DryRun {
				_ = state.DeleteCheckpoint(checkpointName)
				if cfg != nil {
					for _, db := range cfg.Databases {
						_ = os.Remove(filepath.Join(database.SnapshotDir(), db.Engine, checkpointName+".tar"))
						_ = os.Remove(filepath.Join(database.SnapshotDir(), db.Engine, checkpointName+".json"))
					}
				}
				state.CleanupShareDir(bundleID)
			}
		}()

		var dbSnaps []database.SnapshotMeta
		if cfg != nil {
			for _, db := range cfg.Databases {
				if !DryRun {
					if _, err := database.CreateSnapshot(rt, db.Engine, checkpointName); err == nil {
						dbSnaps = append(dbSnaps, database.SnapshotMeta{
							Engine: db.Engine,
							Name:   checkpointName,
						})
					}
				} else {
					dbSnaps = append(dbSnaps, database.SnapshotMeta{Engine: db.Engine, Name: checkpointName})
				}
			}
		}

		// Containers
		if !stateShareDbOnly && !DryRun {
			// Create temp checkpoint
			if err := state.CreateCheckpoint(prov.VM.Name(), checkpointName, rt); err != nil {
				return &devxerr.DevxError{
					ExitCode: devxerr.CodeStateShareNoContainers,
					Message:  "Failed to create container checkpoints.",
					Err:      err,
				}
			}
		}

		if len(dbSnaps) == 0 && stateShareDbOnly {
			return &devxerr.DevxError{
				ExitCode: devxerr.CodeStateShareNoContainers,
				Message:  "Nothing to share — no running containers or databases found.",
			}
		}

		if DryRun {
			dryContainers := 0
			if !stateShareDbOnly {
				dryContainers = 1 // Estimate: at least 1 container in full mode
			}
			if outputJSON {
				out := map[string]interface{}{
					"id":         "dry-run-id",
					"mode":       map[bool]string{true: "db-only", false: "full"}[stateShareDbOnly],
					"databases":  len(dbSnaps),
					"containers": dryContainers,
					"relay":      relayCfg,
				}
				b, _ := json.Marshal(out)
				fmt.Println(string(b))
				return nil
			}
			fmt.Println("Dry run: State would be bundled and uploaded to", relayCfg)
			return nil
		}

		// Bundle
		res, err := state.BundleState(checkpointName, bundleID, dbSnaps, !stateShareDbOnly)
		if err != nil {
			return err
		}

		// Encrypt
		passphrase := state.GeneratePassphrase()
		encryptedPath := filepath.Join(state.ShareDir(), bundleID+".encrypted")
		if err := state.EncryptFile(res.ArchivePath, encryptedPath, passphrase); err != nil {
			return err
		}

		// Upload
		backend, uri, _ := state.ParseRelay(relayCfg)
		uploadURI := uri + "/" + bundleID + ".encrypted"
		switch backend {
		case "s3":
			err = state.UploadToS3(encryptedPath, uploadURI)
		case "gcs":
			err = state.UploadToGCS(encryptedPath, uploadURI)
		}
		if err != nil {
			return &devxerr.DevxError{
				ExitCode: devxerr.CodeStateShareUploadFailed,
				Message:  "Failed to upload to " + backend,
				Err:      err,
			}
		}

		shareToken := fmt.Sprintf("%s:%s", uploadURI, passphrase)

		if outputJSON {
			out := map[string]interface{}{
				"id":         shareToken,
				"mode":       res.Manifest.Mode,
				"databases":  len(res.Manifest.Databases),
				"containers": len(res.Manifest.Containers),
				"size_bytes": res.Manifest.SizeBytes,
				"relay":      relayCfg,
			}
			b, _ := json.Marshal(out)
			fmt.Println(string(b))
			return nil
		}

		fmt.Printf("✅ State successfully bundled, encrypted, and uploaded!\n\n")
		fmt.Printf("Share this ID with your teammate to attach:\n\n")
		fmt.Printf("  %s\n\n", shareToken)
		fmt.Printf("They can run: devx state attach '%s'\n", shareToken)

		return nil
	},
}

func init() {
	stateCmd.AddCommand(stateShareCmd)
	stateShareCmd.Flags().BoolVar(&stateShareDbOnly, "db-only", false, "Share database volumes only (skips container checkpoints)")
	stateShareCmd.Flags().StringVar(&stateShareRelay, "relay", "", "Override relay destination (s3://... or gs://...)")
}
