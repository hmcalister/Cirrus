package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hmcalister/LiteralCloudService/internal/database"
)

// fakeQuerier implements database.Querier, so handlers can be tested without a
// running database. Each field stubs the matching method.
type fakeQuerier struct {
	database.Querier

	createAuthor func(database.CreateAuthorParams) (database.Author, error)
	listAuthors  func() ([]database.Author, error)
	getAuthor    func(int64) (database.Author, error)
}

func (f fakeQuerier) CreateAuthor(_ context.Context, arg database.CreateAuthorParams) (database.Author, error) {
	return f.createAuthor(arg)
}

func (f fakeQuerier) ListAuthors(_ context.Context) ([]database.Author, error) {
	return f.listAuthors()
}

func (f fakeQuerier) GetAuthor(_ context.Context, id int64) (database.Author, error) {
	return f.getAuthor(id)
}

func doRequest(t *testing.T, q database.Querier, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	NewServer(q).Routes().ServeHTTP(w, httptest.NewRequest(method, target, nil))
	return w
}

func TestHandleCreateAuthor(t *testing.T) {
	var got database.CreateAuthorParams
	q := fakeQuerier{createAuthor: func(arg database.CreateAuthorParams) (database.Author, error) {
		got = arg
		return database.Author{ID: 1, Name: arg.Name}, nil
	}}

	w := doRequest(t, q, http.MethodPost, "/authors/Tolkien")

	if w.Code != http.StatusCreated {
		t.Errorf("status = %v, want %v", w.Code, http.StatusCreated)
	}
	if got.Name != "Tolkien" {
		t.Errorf("created author name = %q, want %q", got.Name, "Tolkien")
	}

	var author database.Author
	if err := json.NewDecoder(w.Body).Decode(&author); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if author.ID != 1 {
		t.Errorf("author id = %v, want 1", author.ID)
	}
}

func TestHandleCreateAuthorDatabaseError(t *testing.T) {
	q := fakeQuerier{createAuthor: func(database.CreateAuthorParams) (database.Author, error) {
		return database.Author{}, errors.New("connection refused")
	}}

	w := doRequest(t, q, http.MethodPost, "/authors/Tolkien")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
	// The internal error must not leak to the client.
	if body := w.Body.String(); strings.Contains(body, "connection refused") {
		t.Errorf("response leaked internal error: %v", body)
	}
}

func TestHandleListAuthors(t *testing.T) {
	q := fakeQuerier{listAuthors: func() ([]database.Author, error) {
		return []database.Author{{ID: 1, Name: "Tolkien"}, {ID: 2, Name: "LeGuin"}}, nil
	}}

	w := doRequest(t, q, http.MethodGet, "/authors")

	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
	}
	var authors []database.Author
	if err := json.NewDecoder(w.Body).Decode(&authors); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if len(authors) != 2 {
		t.Fatalf("got %v authors, want 2", len(authors))
	}
	if authors[1].Name != "LeGuin" {
		t.Errorf("second author = %q, want %q", authors[1].Name, "LeGuin")
	}
}

func TestHandleGetAuthorBadID(t *testing.T) {
	// A non-numeric id must be rejected before the database is consulted.
	q := fakeQuerier{getAuthor: func(int64) (database.Author, error) {
		t.Fatal("database should not be queried for a malformed id")
		return database.Author{}, nil
	}}

	w := doRequest(t, q, http.MethodGet, "/authors/not-a-number")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestHandleHealth(t *testing.T) {
	w := doRequest(t, fakeQuerier{}, http.MethodGet, "/healthz")

	if w.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
	}
}
