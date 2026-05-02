package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VitruvianSoftware/devx/internal/logs"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// logsCmd represents the unified terminal log multiplexer command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Unified TUI log multiplexer for containers and native host processes via BubbleTea",
	Long: `Discovers all running containers in the devx VM AND native host processes launched via 'devx run'.
Multiplexes their stdout/stderr into a single unified stream, color-codes them by service name, 
and allows advanced interactive filtering/searching across the entire local stack.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, err := getFullProvider()
		if err != nil {
			return err
		}

		// Idea 38: Initialize secret redactor from current environment
		redactor := logs.NewSecretRedactor()

		if outputJSON {
			// AI Native non-interactive JSON streaming
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			st := logs.NewStreamer(prov.Runtime)
			st.Redactor = redactor
			st.Start(ctx)

			for msg := range st.Lines {
				b, _ := json.Marshal(msg)
				fmt.Println(string(b))
			}
			return nil
		}

		p := tea.NewProgram(logs.InitialModelWithRedactor(redactor, prov.Runtime), tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running log multiplexer: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
