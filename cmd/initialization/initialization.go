package initialization

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/hmcalister/Go-Compose-Template/internal/database"
	"github.com/jackc/pgx/v5"
)

// SetupLogger configures the default slog logger based on the DEBUG environment variable.
// Since this application runs in a container, the DEBUG environment variable is set in the compose.yaml file.
// If DEBUG is not set, logs are written as JSON to /app/logs/log at INFO level.
// If DEBUG is set, logs are written as text to stdout at INFO level.
func SetupLogger() error {
	debugFlag := os.Getenv("DEBUG")
	if debugFlag == "" {
		logFile, err := os.OpenFile("/app/logs/log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			slog.Error("could not create log file", "error", err)
			return err
		}
		// Note: caller is responsible for closing logFile if needed
		defaultLoggerHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
		slog.SetDefault(slog.New(defaultLoggerHandler))
	} else {
		defaultLoggerHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		slog.SetDefault(slog.New(defaultLoggerHandler))
	}
	return nil
}

// GetDatabase establishes a connection to the PostgreSQL database using
// environment variables and returns a Queries instance for database operations.
func GetDatabase() (*database.Queries, error) {
	ctx := context.Background()

	dbHost := os.Getenv("POSTGRES_HOST")
	dbPort := os.Getenv("POSTGRES_PORT")
	dbUser := os.Getenv("POSTGRES_USER")
	dbDatabase := os.Getenv("POSTGRES_DB")
	dbPassword := os.Getenv("POSTGRES_PASSWORD")

	dbConnectionStr := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s", dbUser, dbPassword, dbHost, dbPort, dbDatabase)
	conn, err := pgx.Connect(ctx, dbConnectionStr)
	if err != nil {
		slog.Error("error when connecting to db", "error", err)
		return nil, err
	}
	return database.New(conn), nil
}
