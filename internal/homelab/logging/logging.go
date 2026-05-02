// Package logging provides a structured logger using log/slog for the homelab CLI.
package logging

import (
	"log/slog"
	"os"
)

// Setup initializes the global slog logger with the appropriate level and format.
func Setup(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	slog.SetDefault(slog.New(handler))
}
