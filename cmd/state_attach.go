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

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/state"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var stateAttachCmd = &cobra.Command{
	Use:   "attach <ID>",
	Short: "Download and restore a shared environment state from a teammate",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		idx := strings.LastIndex(id, ":")
		if idx == -1 {
			return &devxerr.DevxError{
				ExitCode: devxerr.CodeStateAttachInvalidID,
				Message:  "Malformed share ID. Expected format: <uri>:<passphrase>",
			}
		}

		uri := id[:idx]
		passphrase := id[idx+1:]

		backend, _, err := state.ParseRelay(uri)
		if err != nil {
			return &devxerr.DevxError{
				ExitCode: devxerr.CodeStateAttachInvalidID,
				Message:  err.Error(),
				Err:      err,
			}
		}

		prov, err := getFullProvider()
		if err != nil {
			return err
		}
		rt := prov.Runtime

		if DryRun {
			if outputJSON {
				b, _ := json.Marshal(map[string]interface{}{
					"backend": backend,
					"uri":     uri,
					"dry_run": true,
				})
				fmt.Println(string(b))
				return nil
			}
			fmt.Printf("Dry run: Would download from %s, decrypt, and restore.\n", backend)
			return nil
		}

		bundleID := uuid.New().String()
		encryptedPath := filepath.Join(state.ShareDir(), bundleID+".encrypted")
		archivePath := filepath.Join(state.ShareDir(), bundleID+".bundle.tar.gz")
		extractDir := filepath.Join(state.ShareDir(), bundleID)

		defer state.CleanupShareDir(bundleID)

		if !outputJSON {
			fmt.Printf("⬇️  Downloading state from %s...\n", backend)
		}

		switch backend {
		case "s3":
			err = state.DownloadFromS3(uri, encryptedPath)
		case "gcs":
			err = state.DownloadFromGCS(uri, encryptedPath)
		}

		if err != nil {
			return &devxerr.DevxError{
				ExitCode: devxerr.CodeStateAttachDownloadFail,
				Message:  "Failed to download bundle",
				Err:      err,
			}
		}

		if !outputJSON {
			fmt.Println("🔓 Decrypting bundle...")
		}

		if err := state.DecryptFile(encryptedPath, archivePath, passphrase); err != nil {
			return &devxerr.DevxError{
				ExitCode: devxerr.CodeStateAttachDecryptFail,
				Message:  "Decryption failed — check your share ID. The passphrase may be incorrect.",
				Err:      err,
			}
		}

		manifest, err := state.UnbundleState(archivePath, extractDir)
		if err != nil {
			return &devxerr.DevxError{
				ExitCode: devxerr.CodeStateAttachDecryptFail,
				Message:  "Failed to extract or parse bundle",
				Err:      err,
			}
		}

		if !outputJSON && !NonInteractive {
			fmt.Printf("\nReady to attach state.\n")
			fmt.Printf("Containers: %d\n", len(manifest.Containers))
			fmt.Printf("Databases: %d\n", len(manifest.Databases))
			fmt.Printf("WARNING: This will overwrite your current environment state.\n\n")
			
			fmt.Printf("Proceed with attach? [y/N]: ")
			var confirm string
			fmt.Scanln(&confirm) //nolint:errcheck
			if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
				return nil
			}
		}

		// Restore CRIU containers
		if manifest.Mode == "full" {
			if rt.Name() != "podman" {
				return &devxerr.DevxError{
					ExitCode: devxerr.CodeStateAttachRestoreFail,
					Message:  "This bundle contains CRIU checkpoints which require Podman, but you are using " + rt.Name(),
				}
			}
			// Copy extracted archives to checkpoint dir (respects DEVX_CHECKPOINT_DIR)
			cpDir := filepath.Join(state.CheckpointsDir(), manifest.CheckpointName)
			_ = os.MkdirAll(cpDir, 0755)
			for _, c := range manifest.Containers {
				src := filepath.Join(extractDir, "containers", c)
				dst := filepath.Join(cpDir, c)
				_ = os.Rename(src, dst)
			}
			
			if err := state.RestoreCheckpoint(prov.VM.Name(), manifest.CheckpointName, rt); err != nil {
				return &devxerr.DevxError{
					ExitCode: devxerr.CodeStateAttachRestoreFail,
					Message:  "Failed to restore containers",
					Err:      err,
				}
			}
		}

		// Restore DBs
		for _, db := range manifest.Databases {
			srcTar := filepath.Join(extractDir, "databases", fmt.Sprintf("%s_%s.tar", db.Engine, db.Name))
			srcJson := filepath.Join(extractDir, "databases", fmt.Sprintf("%s_%s.json", db.Engine, db.Name))
			
			dstDir := filepath.Join(database.SnapshotDir(), db.Engine)
			_ = os.MkdirAll(dstDir, 0755)
			
			_ = os.Rename(srcTar, filepath.Join(dstDir, db.Name+".tar"))
			_ = os.Rename(srcJson, filepath.Join(dstDir, db.Name+".json"))
			
			if err := database.RestoreSnapshot(rt, db.Engine, db.Name); err != nil {
				return &devxerr.DevxError{
					ExitCode: devxerr.CodeStateAttachRestoreFail,
					Message:  "Failed to restore database " + db.Engine,
					Err:      err,
				}
			}
		}

		if outputJSON {
			b, _ := json.Marshal(map[string]interface{}{"success": true})
			fmt.Println(string(b))
			return nil
		}

		fmt.Println("\n✅ State successfully attached! Your environment is now in sync.")
		return nil
	},
}

func init() {
	stateCmd.AddCommand(stateAttachCmd)
}
