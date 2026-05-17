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
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cron jobs declared in devx.yaml",
	RunE:  runCronList,
}

type cronListEntry struct {
	Name        string   `json:"name"`
	Schedule    string   `json:"schedule"`
	Command     []string `json:"command"`
	Description string   `json:"description,omitempty"`
	Timeout     string   `json:"timeout,omitempty"`
}

func init() {
	cronCmd.AddCommand(cronListCmd)
}

func runCronList(_ *cobra.Command, _ []string) error {
	jobs, err := loadCronJobs()
	if err != nil {
		return err
	}

	if outputJSON {
		entries := make([]cronListEntry, 0, len(jobs))
		for _, j := range jobs {
			e := cronListEntry{
				Name:        j.Name,
				Schedule:    j.Schedule,
				Command:     j.Command,
				Description: j.Description,
			}
			if j.Timeout > 0 {
				e.Timeout = j.Timeout.String()
			}
			entries = append(entries, e)
		}
		return json.NewEncoder(os.Stdout).Encode(entries)
	}

	if len(jobs) == 0 {
		fmt.Println("🕐 No cron jobs declared in devx.yaml.")
		fmt.Println("   Add entries under a top-level 'cron:' block to register one.")
		return nil
	}

	fmt.Printf("🕐 %d cron job(s) in devx.yaml:\n\n", len(jobs))
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  NAME\tSCHEDULE\tCOMMAND\tDESCRIPTION")
	for _, j := range jobs {
		desc := j.Description
		if desc == "" {
			desc = "—"
		}
		schedule := j.Schedule
		if schedule == "" {
			schedule = "—"
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
			j.Name, schedule, strings.Join(j.Command, " "), desc)
	}
	_ = tw.Flush()
	fmt.Println("\n  Run one with:  devx cron run <name>")
	return nil
}
