// Package api contains the HTTP routes and handlers for the application.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hmcalister/Cirrus/internal/database"
	"github.com/hmcalister/Cirrus/internal/email/sender"
)

// Server holds the dependencies shared by the handlers.
// Handlers are methods so they can be tested with httptest and a fake Querier.
type Server struct {
	queries database.Querier
	email   sender.Sender
}

func NewServer(queries database.Querier, emailSender sender.Sender) *Server {
	return &Server{queries: queries, email: emailSender}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("error writing response body", "error", err)
	}
}
