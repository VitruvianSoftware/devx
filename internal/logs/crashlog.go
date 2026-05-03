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

package logs

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// crashBoxStyle renders the crash log in a high-visibility box.
var crashBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#FF4444")).
	Padding(0, 1).
	MarginTop(1).
	MarginBottom(1)

var crashHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FF4444"))

var crashLineStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#CCCCCC"))

// TailContainerCrashLogs fetches the last N lines from a crashed container and
// prints them in a styled error box for immediate developer context.
func TailContainerCrashLogs(runtime, containerName string, lines int) {
	out, err := exec.Command(runtime, "logs", "--tail", fmt.Sprintf("%d", lines), containerName).CombinedOutput()
	if err != nil || len(out) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "  (could not retrieve logs for %s)\n", containerName)
		return
	}

	renderCrashBox(containerName, "container", string(out))
}

// TailHostCrashLogs reads the last N lines from a host process log file and
// prints them in a styled error box.
func TailHostCrashLogs(serviceName string, lines int) {
	logPath := filepath.Join(os.Getenv("HOME"), ".devx", "logs", serviceName+".log")

	f, err := os.Open(logPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "  (could not retrieve logs for %s: %v)\n", serviceName, err)
		return
	}
	defer func() { _ = f.Close() }() 

	// Read all lines, then take last N
	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	start := 0
	if len(allLines) > lines {
		start = len(allLines) - lines
	}

	renderCrashBox(serviceName, "host", strings.Join(allLines[start:], "\n"))
}

// renderCrashBox prints the formatted crash log output.
func renderCrashBox(name, logType, output string) {
	header := crashHeaderStyle.Render(
		fmt.Sprintf("💥 %s (%s) crashed — last log output:", name, logType),
	)

	// Dim each log line for readability
	var styledLines []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		styledLines = append(styledLines, crashLineStyle.Render(line))
	}

	body := strings.Join(styledLines, "\n")
	box := crashBoxStyle.Render(header + "\n\n" + body)

	_, _ = fmt.Fprintln(os.Stderr, box)
}
