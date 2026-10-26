package logging

import (
	"log/slog"
	"os"
)

// Configure the default logger.
//
// If debug is set, logs are DEBUG-level human-readable text.
// Otherwise, logs are INFO-level JSON.
func Setup(debug bool) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, opts)
	if debug {
		opts.Level = slog.LevelDebug
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
