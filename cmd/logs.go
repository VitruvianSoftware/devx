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
	GroupID: "orchestration",
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
