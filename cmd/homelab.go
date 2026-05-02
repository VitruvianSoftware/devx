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
	"github.com/spf13/cobra"
)

var homelabConfigFile string

var homelabCmd = &cobra.Command{
	Use:   "homelab",
	GroupID: "infra",
	Short: "Declaratively provision and manage multi-node K8s homelab clusters on macOS",
	Long: `homelab is a CLI tool for provisioning and managing Kubernetes clusters
across multiple macOS machines using Lima VZ and K3s.

Define your cluster topology in a YAML config file, then use homelab to
bootstrap, scale, diagnose, and tear down your cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	homelabCmd.PersistentFlags().StringVarP(&homelabConfigFile, "config", "c", "homelab.yaml", "path to cluster config file")

	homelabCmd.AddCommand(
		newHomelabInitCmd(&homelabConfigFile),
		newHomelabApplyCmd(&homelabConfigFile),
		newHomelabJoinCmd(&homelabConfigFile),
		newHomelabRemoveCmd(&homelabConfigFile),
		newHomelabDestroyCmd(&homelabConfigFile),
		newHomelabUpgradeCmd(&homelabConfigFile),
		newHomelabBackupCmd(&homelabConfigFile),
		newHomelabRestoreCmd(&homelabConfigFile),
		newHomelabDoctorCmd(&homelabConfigFile),
		newHomelabStatusCmd(&homelabConfigFile),
	)

}
