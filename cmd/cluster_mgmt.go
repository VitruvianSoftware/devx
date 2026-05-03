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

var clusterConfigFile string

var clusterMgmtCmd = &cobra.Command{
	Use:   "cluster",
	GroupID: "k8s",
	Short: "Provision and manage multi-node K8s clusters across macOS machines",
	Long: `cluster is a CLI tool for provisioning and managing Kubernetes clusters
across multiple macOS machines using Lima VZ and K3s.

Define your cluster topology in a YAML config file, then use cluster to
bootstrap, scale, diagnose, and tear down your cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	clusterMgmtCmd.PersistentFlags().StringVarP(&clusterConfigFile, "config", "c", "cluster.yaml", "path to cluster config file")

	clusterMgmtCmd.AddCommand(
		newClusterInitCmd(&clusterConfigFile),
		newClusterApplyCmd(&clusterConfigFile),
		newClusterJoinCmd(&clusterConfigFile),
		newClusterRemoveCmd(&clusterConfigFile),
		newClusterDestroyCmd(&clusterConfigFile),
		newClusterUpgradeCmd(&clusterConfigFile),
		newClusterBackupCmd(&clusterConfigFile),
		newClusterRestoreCmd(&clusterConfigFile),
		newClusterDoctorCmd(&clusterConfigFile),
		newClusterStatusCmd(&clusterConfigFile),
	)

}
