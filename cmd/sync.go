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

import "github.com/spf13/cobra"

var syncCmd = &cobra.Command{
	Use:   "sync",
	GroupID: "orchestration",
	Short: "Manage intelligent file synchronization into containers",
	Long: `Bypass slow VirtioFS volume mounts by syncing file changes directly
into running containers via Mutagen. Changes propagate in milliseconds.

Sync sessions run as persistent background processes and survive terminal exit.
Use 'devx sync rm' to clean up when finished.

Quick start:
  devx sync up          # start all sync sessions from devx.yaml
  devx sync list        # show active sessions
  devx sync rm          # terminate all sessions`,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
