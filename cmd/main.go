package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/hmcalister/LiteralCloudService/cmd/initialization"
)

func main() {
	if err := initialization.SetupLogger(); err != nil {
		os.Exit(1)
	}

	db, err := initialization.GetDatabase()
	if err != nil {
		slog.Error("error while initializing database", "error", err)
		os.Exit(1)
	}

	// --------------------------------------------------------------------------------

	mux := http.NewServeMux()

	slog.Info("ready to serve")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		slog.Error("error during listen and serve", "error", err)
	}
}
