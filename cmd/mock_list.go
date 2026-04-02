package cmd

import (
	"encoding/json"
	"fmt"

	devxmock "github.com/VitruvianSoftware/devx/internal/mock"
	"github.com/spf13/cobra"
)

var mockListRuntime string

var mockListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all running devx mock servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime := mockListRuntime
		infos, err := devxmock.List(runtime)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(infos)
		}

		if len(infos) == 0 {
			fmt.Println("No mock servers running. Start one with: devx mock up")
			return nil
		}

		fmt.Printf("%-16s %-8s %-44s %s\n", "NAME", "PORT", "ENV VAR INJECTION", "STATUS")
		fmt.Printf("%-16s %-8s %-44s %s\n", "----", "----", "-----------------", "------")
		for _, m := range infos {
			fmt.Printf("%-16s %-8d %-44s %s\n", m.Name, m.Port, m.EnvVar, m.Status)
		}
		return nil
	},
}

func init() {
	mockListCmd.Flags().StringVar(&mockListRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	mockCmd.AddCommand(mockListCmd)
}
