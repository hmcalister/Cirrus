// Package api contains the HTTP routes and handlers for the application.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/hmcalister/Cirrus/internal/database"
	"github.com/hmcalister/Cirrus/internal/email"
)

// Server holds the dependencies shared by the handlers.
// Handlers are methods so they can be tested with httptest and a fake Querier.
type Server struct {
	queries database.Querier
	email   email.Sender
}

func NewServer(queries database.Querier, emailSender email.Sender) *Server {
	return &Server{queries: queries, email: emailSender}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /authors/{authorName}", s.handleCreateAuthor)
	mux.HandleFunc("GET /authors", s.handleListAuthors)
	mux.HandleFunc("GET /authors/{authorID}", s.handleGetAuthor)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateAuthor(w http.ResponseWriter, r *http.Request) {
	authorName := r.PathValue("authorName")
	slog.Info("new author request", "authorName", authorName)

	author, err := s.queries.CreateAuthor(r.Context(), database.CreateAuthorParams{
		Name: authorName,
	})
	if err != nil {
		slog.Error("error when creating new author", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create author"})
		return
	}

	writeJSON(w, http.StatusCreated, author)
}

func (s *Server) handleListAuthors(w http.ResponseWriter, r *http.Request) {
	slog.Info("all authors request")

	allAuthors, err := s.queries.ListAuthors(r.Context())
	if err != nil {
		slog.Error("error when requesting authors", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list authors"})
		return
	}

	writeJSON(w, http.StatusOK, allAuthors)
}

func (s *Server) handleGetAuthor(w http.ResponseWriter, r *http.Request) {
	authorID, err := strconv.ParseInt(r.PathValue("authorID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "author id must be an integer"})
		return
	}
	slog.Info("author request", "authorID", authorID)

	author, err := s.queries.GetAuthor(r.Context(), authorID)
	if err != nil {
		slog.Error("error when requesting author", "error", err, "authorID", authorID)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "author not found"})
		return
	}

	writeJSON(w, http.StatusOK, author)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("error writing response body", "error", err)
	}
}
