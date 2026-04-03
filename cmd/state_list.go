package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/VitruvianSoftware/devx/internal/state"
	"github.com/spf13/cobra"
)

var stateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all time-travel checkpoints",
	Long:  `Lists all locally stored CRIU checkpoints created via devx state checkpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := state.ListCheckpoints()
		if err != nil {
			return err
		}

		if outputJSON {
			b, _ := json.MarshalIndent(names, "", "  ")
			fmt.Println(string(b))
			return nil
		}

		if len(names) == 0 {
			fmt.Println("No time-travel checkpoints found.")
			fmt.Println("Create one with: devx state checkpoint <name>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "CHECKPOINT NAME")
		fmt.Fprintln(w, "───────────────")
		for _, n := range names {
			fmt.Fprintln(w, n)
		}
		return w.Flush()
	},
}

func init() {
	stateCmd.AddCommand(stateListCmd)
}
