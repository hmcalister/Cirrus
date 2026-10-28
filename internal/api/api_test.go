package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/cirrus/internal/database"
	"github.com/hmcalister/cirrus/internal/filestore"
	skysatchel "github.com/hmcalister/skysatchel/pkg/email"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
	"github.com/hmcalister/stratus/pkg/auth"
	"github.com/hmcalister/stratus/pkg/sessionstore"
)

const (
	testEmail = "user@example.com"
	testToken = "raw-session-token"
)

type stubAuthService struct {
	identity     auth.Identity
	validateErr  error
	logoutAllErr error

	loggedOutAll []string
}

func (s *stubAuthService) ValidateSession(_ context.Context, rawToken string) (auth.Identity, error) {
	if s.validateErr != nil {
		return auth.Identity{}, s.validateErr
	}
	if rawToken == "" {
		return auth.Identity{}, auth.ErrUnauthenticated
	}
	return s.identity, nil
}

func (s *stubAuthService) LogoutAll(_ context.Context, userEmail string) error {
	s.loggedOutAll = append(s.loggedOutAll, userEmail)
	return s.logoutAllErr
}

func (s *stubAuthService) RegisterHumaHandlers(huma.API) {}

var _ authService = (*stubAuthService)(nil)

type stubUserStore struct {
	users        map[string]database.UserAccount
	getErr       error
	setActiveErr error
	deleteErr    error
	countErr     error

	deleted []string
}

func newStubUserStore() *stubUserStore {
	return &stubUserStore{users: make(map[string]database.UserAccount)}
}

func (s *stubUserStore) GetOrCreateUser(_ context.Context, email string) (database.UserAccount, error) {
	if s.getErr != nil {
		return database.UserAccount{}, s.getErr
	}
	if s.users == nil {
		s.users = make(map[string]database.UserAccount)
	}

	user, ok := s.users[email]
	if !ok {
		user = database.UserAccount{Email: email, Active: true}
		s.users[email] = user
	}
	return user, nil
}

func (s *stubUserStore) SetActive(_ context.Context, arg database.SetActiveParams) error {
	if s.setActiveErr != nil {
		return s.setActiveErr
	}

	user := s.users[arg.Email]
	user.Email = arg.Email
	user.Active = arg.Active
	s.users[arg.Email] = user
	return nil
}

func (s *stubUserStore) DeleteUser(_ context.Context, email string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}

	delete(s.users, email)
	s.deleted = append(s.deleted, email)
	return nil
}

func (s *stubUserStore) CountUsers(_ context.Context) (int64, error) {
	if s.countErr != nil {
		return 0, s.countErr
	}
	return int64(len(s.users)), nil
}

func (s *stubUserStore) CountActiveUsers(_ context.Context) (int64, error) {
	if s.countErr != nil {
		return 0, s.countErr
	}

	var count int64
	for _, user := range s.users {
		if user.Active {
			count++
		}
	}
	return count, nil
}

var _ userStore = (*stubUserStore)(nil)

type stubFileStore struct {
	files []string
	err   error
}

func (s *stubFileStore) ListFiles(_ context.Context) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.files, nil
}

func (s *stubFileStore) GetFile(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}

func (s *stubFileStore) DeleteFile(_ context.Context, _ string) error {
	return nil
}

var _ filestore.FileStore = (*stubFileStore)(nil)

func newTestServer(t *testing.T, authService *stubAuthService, users *stubUserStore) *Server {
	t.Helper()

	if authService == nil {
		authService = &stubAuthService{}
	}
	if users == nil {
		users = newStubUserStore()
	}

	return newTestServerWith(t, ServerConfig{
		AuthService:       authService,
		Users:             users,
		Files:             &stubFileStore{},
		SessionCookieName: secureSessionCookieName,
	})
}

func newTestServerWith(t *testing.T, cfg ServerConfig) *Server {
	t.Helper()

	mux := http.NewServeMux()
	humaAPI := humago.New(mux, NewHumaConfig())
	cfg.AuthService.RegisterHumaHandlers(humaAPI)

	server, err := NewServer(mux, humaAPI, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func doRequest(t *testing.T, server *Server, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)
	return recorder
}

func getWithCookie(t *testing.T, server *Server, path, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		request.AddCookie(&http.Cookie{Name: secureSessionCookieName, Value: cookie})
	}
	return doRequest(t, server, request)
}

func postJSONWithCookie(t *testing.T, server *Server, path string, body any, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(encoded)))
	request.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		request.AddCookie(&http.Cookie{Name: secureSessionCookieName, Value: cookie})
	}
	return doRequest(t, server, request)
}

func deleteWithCookie(t *testing.T, server *Server, path, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodDelete, path, nil)
	if cookie != "" {
		request.AddCookie(&http.Cookie{Name: secureSessionCookieName, Value: cookie})
	}
	return doRequest(t, server, request)
}

func decodeBody[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var value T
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode body %q: %v", recorder.Body.String(), err)
	}
	return value
}

func TestNewServerValidation(t *testing.T) {
	valid := ServerConfig{
		AuthService:       &stubAuthService{},
		Users:             newStubUserStore(),
		Files:             &stubFileStore{},
		SessionCookieName: secureSessionCookieName,
	}

	tests := []struct {
		name   string
		mutate func(*ServerConfig)
	}{
		{"nil auth service", func(c *ServerConfig) { c.AuthService = nil }},
		{"nil user store", func(c *ServerConfig) { c.Users = nil }},
		{"nil file store", func(c *ServerConfig) { c.Files = nil }},
		{"empty cookie name", func(c *ServerConfig) { c.SessionCookieName = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)

			mux := http.NewServeMux()
			humaAPI := humago.New(mux, NewHumaConfig())
			if _, err := NewServer(mux, humaAPI, cfg); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}

	mux := http.NewServeMux()
	humaAPI := humago.New(mux, NewHumaConfig())
	if _, err := NewServer(mux, humaAPI, valid); err != nil {
		t.Errorf("NewServer(valid) = %v, want nil", err)
	}
}

func TestHealth(t *testing.T) {
	server := newTestServer(t, nil, nil)

	recorder := getWithCookie(t, server, "/healthz", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	body := decodeBody[struct {
		Status string `json:"status"`
	}](t, recorder)
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
}

func TestAccountRequiresSession(t *testing.T) {
	server := newTestServer(t, nil, nil)

	recorder := getWithCookie(t, server, "/api/account", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestAccountCreatesProfileAndReturnsJSON(t *testing.T) {
	users := newStubUserStore()
	server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, users)

	recorder := getWithCookie(t, server, "/api/account", testToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	body := decodeBody[struct {
		Email        string    `json:"email"`
		Active       bool      `json:"active"`
		NextCloudsAt time.Time `json:"next_clouds_at"`
	}](t, recorder)
	if body.Email != testEmail {
		t.Errorf("email = %q, want %q", body.Email, testEmail)
	}
	if !body.Active {
		t.Error("new profile should be active")
	}
	if body.NextCloudsAt.IsZero() {
		t.Error("next_clouds_at is missing")
	}
	if _, ok := users.users[testEmail]; !ok {
		t.Error("request did not create a user profile")
	}
}

func TestAccountReportsNextCloudsAt(t *testing.T) {
	users := newStubUserStore()
	server := newTestServerWith(t, ServerConfig{
		AuthService:       &stubAuthService{identity: auth.Identity{Email: testEmail}},
		Users:             users,
		Files:             &stubFileStore{},
		SessionCookieName: secureSessionCookieName,
		DailyEmailTime:    config.DailyEmailTimeConfig{Hour: 8, Minute: 0},
	})
	server.now = func() time.Time {
		return time.Date(2026, time.October, 26, 7, 30, 0, 0, time.UTC)
	}

	recorder := getWithCookie(t, server, "/api/account", testToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	body := decodeBody[struct {
		NextCloudsAt time.Time `json:"next_clouds_at"`
	}](t, recorder)
	want := time.Date(2026, time.October, 26, 8, 0, 0, 0, time.UTC)
	if !body.NextCloudsAt.Equal(want) {
		t.Errorf("next_clouds_at = %v, want %v", body.NextCloudsAt, want)
	}
}

func TestSetActive(t *testing.T) {
	users := newStubUserStore()
	users.users[testEmail] = database.UserAccount{Email: testEmail, Active: true}
	server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, users)

	recorder := postJSONWithCookie(t, server, "/api/account/active", map[string]bool{"active": false}, testToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	body := decodeBody[struct {
		Active bool `json:"active"`
	}](t, recorder)
	if body.Active {
		t.Error("active = true, want false")
	}
	if users.users[testEmail].Active {
		t.Error("user store was not updated")
	}
}

func TestDeleteAccount(t *testing.T) {
	authService := &stubAuthService{identity: auth.Identity{Email: testEmail}}
	users := newStubUserStore()
	users.users[testEmail] = database.UserAccount{Email: testEmail, Active: true}
	server := newTestServer(t, authService, users)

	recorder := deleteWithCookie(t, server, "/api/account", testToken)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	if len(users.deleted) != 1 || users.deleted[0] != testEmail {
		t.Errorf("DeleteUser calls = %v, want [%q]", users.deleted, testEmail)
	}
	if len(authService.loggedOutAll) != 1 || authService.loggedOutAll[0] != testEmail {
		t.Errorf("LogoutAll calls = %v, want [%q]", authService.loggedOutAll, testEmail)
	}
}

func TestDeleteKeepsProfileWhenSessionRevocationFails(t *testing.T) {
	authService := &stubAuthService{
		identity:     auth.Identity{Email: testEmail},
		logoutAllErr: errors.New("store unavailable"),
	}
	users := newStubUserStore()
	users.users[testEmail] = database.UserAccount{Email: testEmail, Active: true}
	server := newTestServer(t, authService, users)

	recorder := deleteWithCookie(t, server, "/api/account", testToken)
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if len(users.deleted) != 0 {
		t.Errorf("profile was deleted although sessions could not be revoked: %v", users.deleted)
	}
}

func TestProfileCreationFailureIsInternalError(t *testing.T) {
	users := newStubUserStore()
	users.getErr = errors.New("database down")
	server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, users)

	recorder := getWithCookie(t, server, "/api/account", testToken)
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestAdminStats(t *testing.T) {
	users := newStubUserStore()
	users.users["admin@example.com"] = database.UserAccount{Email: "admin@example.com", Active: true, Roles: []string{"admin"}}
	users.users["active@example.com"] = database.UserAccount{Email: "active@example.com", Active: true}
	users.users["paused@example.com"] = database.UserAccount{Email: "paused@example.com", Active: false}

	server := newTestServerWith(t, ServerConfig{
		AuthService:       &stubAuthService{identity: auth.Identity{Email: "admin@example.com"}},
		Users:             users,
		Files:             &stubFileStore{files: []string{"one.jpg", "two.jpg"}},
		SessionCookieName: secureSessionCookieName,
	})

	recorder := getWithCookie(t, server, "/api/admin/stats", testToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	body := decodeBody[AdminStats](t, recorder)
	if body.UserAccounts != 3 {
		t.Errorf("user accounts = %d, want 3", body.UserAccounts)
	}
	if body.ActiveUsers != 2 {
		t.Errorf("active users = %d, want 2", body.ActiveUsers)
	}
	if body.RemainingFiles != 2 {
		t.Errorf("remaining files = %d, want 2", body.RemainingFiles)
	}
}

func TestAdminStatsForbiddenForStandardUser(t *testing.T) {
	server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, nil)

	recorder := getWithCookie(t, server, "/api/admin/stats", testToken)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestAdminStatsFailureIsInternalError(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*stubUserStore, *stubFileStore)
	}{
		{"user count", func(users *stubUserStore, _ *stubFileStore) { users.countErr = errors.New("database down") }},
		{"file listing", func(_ *stubUserStore, files *stubFileStore) { files.err = errors.New("bucket down") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users := newStubUserStore()
			users.users["admin@example.com"] = database.UserAccount{Email: "admin@example.com", Active: true, Roles: []string{"admin"}}
			files := &stubFileStore{}
			tc.mutate(users, files)

			server := newTestServerWith(t, ServerConfig{
				AuthService:       &stubAuthService{identity: auth.Identity{Email: "admin@example.com"}},
				Users:             users,
				Files:             files,
				SessionCookieName: secureSessionCookieName,
			})

			recorder := getWithCookie(t, server, "/api/admin/stats", testToken)
			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
			}
		})
	}
}

// Registers a test-only operation behind the given middleware chain, so the
// role middleware can be exercised through the real routing and session
// validation.
func registerTestOperation(server *Server, operationID, path string, middlewares huma.Middlewares) {
	huma.Register(server.api, huma.Operation{
		OperationID:   operationID,
		Method:        http.MethodGet,
		Path:          path,
		DefaultStatus: http.StatusOK,
		Middlewares:   middlewares,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})
}

func TestRequiredRoleMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{"holds role", []string{"admin"}, http.StatusOK},
		{"lacks role", []string{"other"}, http.StatusForbidden},
		{"no roles", nil, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users := newStubUserStore()
			users.users[testEmail] = database.UserAccount{Email: testEmail, Active: true, Roles: tc.roles}
			server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, users)
			registerTestOperation(server, "test-required-role", "/api/test-required-role", huma.Middlewares{
				server.requireUser,
				server.makeRequiredRoleMiddleware("admin"),
			})

			recorder := getWithCookie(t, server, "/api/test-required-role", testToken)
			if recorder.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
		})
	}
}

func TestRequiredRoleMiddlewareRequiresSession(t *testing.T) {
	server := newTestServer(t, nil, nil)
	registerTestOperation(server, "test-required-role-session", "/api/test-required-role-session", huma.Middlewares{
		server.requireUser,
		server.makeRequiredRoleMiddleware("admin"),
	})

	recorder := getWithCookie(t, server, "/api/test-required-role-session", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRequiredRoleMiddlewareWithoutProfileIsInternalError(t *testing.T) {
	server := newTestServer(t, nil, nil)
	registerTestOperation(server, "test-required-role-no-profile", "/api/test-required-role-no-profile", huma.Middlewares{
		server.makeRequiredRoleMiddleware("admin"),
	})

	recorder := getWithCookie(t, server, "/api/test-required-role-no-profile", "")
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

type captureSender struct {
	sent []skysender.SendParams
}

func (s *captureSender) Send(_ context.Context, params skysender.SendParams) error {
	s.sent = append(s.sent, params)
	return nil
}

var _ skysender.Sender = (*captureSender)(nil)

// Exercises the real Stratus service end to end: request a link, consume it,
// use the session, then sign out.
func TestMagicLinkSignInFlow(t *testing.T) {
	sender := &captureSender{}
	authService, err := auth.New(auth.AuthServiceConfig{
		Sender:     sender,
		Sessions:   sessionstore.NewMemorySessionStore(),
		BaseURL:    "https://cloud.example.com",
		SuccessURL: "/account",
		Cookie: auth.CookieConfig{
			Name:   secureSessionCookieName,
			Secure: true,
		},
		LinkEmail: func(link string) skysatchel.Message {
			return skysatchel.Message{Subject: "Sign in", Body: link}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	users := newStubUserStore()
	server := newTestServerWith(t, ServerConfig{
		AuthService:       authService,
		Users:             users,
		Files:             &stubFileStore{},
		SessionCookieName: secureSessionCookieName,
	})

	recorder := postJSONWithCookie(t, server, "/auth/magic-link", map[string]string{"email": testEmail}, "")
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("magic-link request status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent %d emails, want 1", len(sender.sent))
	}

	link, err := url.Parse(sender.sent[0].Body)
	if err != nil {
		t.Fatalf("parsing link %q: %v", sender.sent[0].Body, err)
	}
	consume := httptest.NewRequest(http.MethodGet, link.RequestURI(), nil)
	consumeRecorder := doRequest(t, server, consume)

	if consumeRecorder.Code != http.StatusFound {
		t.Fatalf("consume status = %d, want %d", consumeRecorder.Code, http.StatusFound)
	}
	if got := consumeRecorder.Header().Get("Location"); got != "/account" {
		t.Errorf("consume Location = %q, want /account", got)
	}

	var sessionCookie *http.Cookie
	for _, cookie := range consumeRecorder.Result().Cookies() {
		if cookie.Name == secureSessionCookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie was not set")
	}

	accountRecorder := getWithCookie(t, server, "/api/account", sessionCookie.Value)
	if accountRecorder.Code != http.StatusOK {
		t.Fatalf("account status = %d, want %d: %s", accountRecorder.Code, http.StatusOK, accountRecorder.Body.String())
	}
	if _, ok := users.users[testEmail]; !ok {
		t.Error("sign in did not create a user profile")
	}
}

func TestOpenAPIDocumentsOperations(t *testing.T) {
	server := newTestServer(t, &stubAuthService{}, nil)

	paths := server.OpenAPI().Paths
	for _, path := range []string{"/healthz", "/api/account", "/api/account/active", "/api/admin/stats"} {
		if paths[path] == nil {
			t.Errorf("OpenAPI document is missing path %q", path)
		}
	}
	if paths["/api/account"] != nil && paths["/api/account"].Delete == nil {
		t.Error("OpenAPI document is missing DELETE /api/account")
	}
}
