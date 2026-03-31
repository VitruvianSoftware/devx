package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

var runName string

var runCmd = &cobra.Command{
	Use:   "run [command...]",
	Short: "Run a native host process and route its output to the unified devx log stream",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := runName
		if name == "" {
			name = filepath.Base(args[0])
		}

		// Ensure log dir exists
		logDir := filepath.Join(os.Getenv("HOME"), ".devx", "logs")
		os.MkdirAll(logDir, 0755)

		// Open log file for append
		logFile, err := os.OpenFile(filepath.Join(logDir, name+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Printf("Warning: Could not open log file: %v\n", err)
		} else {
			defer logFile.Close()
		}

		fmt.Printf("Started native process [%s]. Logs routing to devx logs...\n", name)

		command := exec.Command(args[0], args[1:]...)
		
		// Setup multi-writers to route to both terminal and log file
		if logFile != nil {
			command.Stdout = io.MultiWriter(os.Stdout, logFile)
			command.Stderr = io.MultiWriter(os.Stderr, logFile)
		} else {
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
		}
		command.Stdin = os.Stdin

		err = command.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start process: %v\n", err)
			os.Exit(1)
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			command.Process.Signal(syscall.SIGINT)
		}()

		err = command.Wait()
		if err != nil {
			// Expected for interrupted commands
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runName, "name", "n", "", "Custom name for the log stream tail (defaults to executable name)")
}
