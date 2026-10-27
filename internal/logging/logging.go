package logging

import (
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Path to log file on container
const logFilePath = "/var/log/cirrus/cirrus.log"

// Configure the default logger.
//
// Records go to stdout and to the log file.
// If debug is set, logs are DEBUG-level human-readable text. Otherwise, logs are INFO-level JSON.
func Setup(debug bool) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	file := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10,
		MaxBackups: 14,
		MaxAge:     30,
		Compress:   true,
	}

	var stdout slog.Handler = slog.NewJSONHandler(os.Stdout, opts)
	var logfile slog.Handler = slog.NewJSONHandler(file, opts)
	if debug {
		opts.Level = slog.LevelDebug
		stdout = slog.NewTextHandler(os.Stdout, opts)
		logfile = slog.NewTextHandler(file, opts)
	}

	slog.SetDefault(slog.New(slog.NewMultiHandler(stdout, logfile)))
}
