// Package logging configures the process-wide slog logger.
package logging

import (
	"log/slog"
	"os"
)

// Setup installs the default logger.
//
// In debug mode logs are human-readable text at DEBUG level, otherwise JSON at INFO level.
func Setup(debug bool) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, opts)
	if debug {
		opts.Level = slog.LevelDebug
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
