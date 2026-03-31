package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
	Run: func(cmd *cobra.Command, args []string) {
		if outputJSON {
			// AI Native non-interactive JSON streaming
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			st := logs.NewStreamer()
			st.Start(ctx)
			
			for msg := range st.Lines {
				b, _ := json.Marshal(msg)
				fmt.Println(string(b))
			}
			return
		}

		p := tea.NewProgram(logs.InitialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running log multiplexer: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
