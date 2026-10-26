package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hmcalister/cirrus/internal/database"
	templates "github.com/hmcalister/cirrus/internal/template"
	skysatchel "github.com/hmcalister/skysatchel/pkg/email"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
	"github.com/hmcalister/stratus/pkg/auth"
	"github.com/hmcalister/stratus/pkg/sessionstore"
)

const (
	testEmail = "user@example.com"
	testToken = "raw-session-token"
)

// Stub of the authService seam. Records the calls it receives and returns the
// behaviour configured by the test.
type stubAuthService struct {
	identity     auth.Identity
	validateErr  error
	logoutErr    error
	logoutAllErr error

	loggedOut    []string
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

func (s *stubAuthService) Logout(_ context.Context, rawToken string) error {
	s.loggedOut = append(s.loggedOut, rawToken)
	return s.logoutErr
}

func (s *stubAuthService) LogoutAll(_ context.Context, userEmail string) error {
	s.loggedOutAll = append(s.loggedOutAll, userEmail)
	return s.logoutAllErr
}

var _ authService = (*stubAuthService)(nil)

// Stub of the user profile store.
type stubUserStore struct {
	users        map[string]database.UserAccount
	getErr       error
	setActiveErr error
	deleteErr    error

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

var _ userStore = (*stubUserStore)(nil)

// Stub page renderer. Produces a body derived from the template name and data
// so tests can assert the rendered fragment without owning real templates.
type stubRenderer struct {
	err error
}

func (s stubRenderer) RenderPage(templateName string, data any) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return fmt.Sprintf("<rendered %s %v>", templateName, data), nil
}

var _ templates.PageRenderer = stubRenderer{}

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
		PagesRenderer:     stubRenderer{},
		CookieSecure:      true,
		SessionCookieName: secureSessionCookieName,
	})
}

func newTestServerWith(t *testing.T, cfg ServerConfig) *Server {
	t.Helper()

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

// Posts url.Values to path through the server's routes.
func postForm(t *testing.T, server *Server, path string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return postWithCookie(t, server, path, values, "")
}

// Posts url.Values to path carrying the given session cookie.
func postWithCookie(t *testing.T, server *Server, path string, values url.Values, cookieValue string) *httptest.ResponseRecorder {
	t.Helper()

	body := ""
	if values != nil {
		body = values.Encode()
	}

	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookieValue != "" {
		request.AddCookie(&http.Cookie{Name: secureSessionCookieName, Value: cookieValue})
	}
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
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)
	return recorder
}

// postAuthedForm issues a POST carrying a session cookie so the request passes
// RequireAuth.
func postAuthedForm(t *testing.T, server *Server, path string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return postWithCookie(t, server, path, values, testToken)
}

func sessionCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == secureSessionCookieName {
			return cookie
		}
	}
	t.Fatalf("response has no %q cookie; headers: %v", secureSessionCookieName, recorder.Header())
	return nil
}

func TestNewServerValidation(t *testing.T) {
	valid := ServerConfig{
		AuthService:       &stubAuthService{},
		Users:             newStubUserStore(),
		PagesRenderer:     stubRenderer{},
		SessionCookieName: secureSessionCookieName,
	}

	tests := []struct {
		name   string
		mutate func(*ServerConfig)
	}{
		{"nil auth service", func(c *ServerConfig) { c.AuthService = nil }},
		{"nil user store", func(c *ServerConfig) { c.Users = nil }},
		{"nil pages renderer", func(c *ServerConfig) { c.PagesRenderer = nil }},
		{"empty cookie name", func(c *ServerConfig) { c.SessionCookieName = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			if _, err := NewServer(cfg); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}

	if _, err := NewServer(valid); err != nil {
		t.Errorf("NewServer(valid) = %v, want nil", err)
	}
}

func TestMountedAuthHandlersServe(t *testing.T) {
	authRoutes := http.NewServeMux()
	authRoutes.HandleFunc("POST /auth/magic-link", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	server := newTestServerWith(t, ServerConfig{
		AuthService:       &stubAuthService{},
		Users:             newStubUserStore(),
		AuthHandlers:      authRoutes,
		PagesRenderer:     stubRenderer{},
		SessionCookieName: secureSessionCookieName,
	})

	recorder := postForm(t, server, "/auth/magic-link", url.Values{"email": {testEmail}})
	if recorder.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

// Captures the messages a Sender would deliver.
type captureSender struct {
	sent []skysender.SendParams
}

func (s *captureSender) Send(_ context.Context, params skysender.SendParams) error {
	s.sent = append(s.sent, params)
	return nil
}

var _ skysender.Sender = (*captureSender)(nil)

// Exercises the real Stratus service end to end: request a link, consume it,
// use the session, then sign out. This pins the cookie name shared between
// Stratus and RequireAuth.
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
		AuthHandlers:      authService.AuthHandlers(),
		PagesRenderer:     stubRenderer{},
		CookieSecure:      true,
		SessionCookieName: secureSessionCookieName,
	})

	// 1. Request a magic link.
	recorder := postForm(t, server, "/auth/magic-link", url.Values{"email": {testEmail}})
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("magic-link request status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent %d emails, want 1", len(sender.sent))
	}
	if got := sender.sent[0].ToRecipients; len(got) != 1 || got[0] != testEmail {
		t.Fatalf("ToRecipients = %v, want [%s]", got, testEmail)
	}

	// 2. Consume the link. Stratus sets the session cookie and redirects.
	link, err := url.Parse(sender.sent[0].Body)
	if err != nil {
		t.Fatalf("parsing link %q: %v", sender.sent[0].Body, err)
	}
	consume := httptest.NewRequest(http.MethodGet, link.RequestURI(), nil)
	consumeRecorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(consumeRecorder, consume)

	if consumeRecorder.Code != http.StatusFound {
		t.Fatalf("consume status = %d, want %d", consumeRecorder.Code, http.StatusFound)
	}
	if got := consumeRecorder.Header().Get("Location"); got != "/account" {
		t.Errorf("consume Location = %q, want /account", got)
	}
	cookie := sessionCookie(t, consumeRecorder)

	// 3. The session reaches a protected page and creates the profile.
	accountRecorder := getWithCookie(t, server, "/account", cookie.Value)
	if accountRecorder.Code != http.StatusOK {
		t.Fatalf("account status = %d, want %d", accountRecorder.Code, http.StatusOK)
	}
	if _, ok := users.users[testEmail]; !ok {
		t.Error("sign in did not create a user profile")
	}

	// 4. Signing out revokes the session.
	logoutRecorder := postWithCookie(t, server, "/logout", nil, cookie.Value)
	if logoutRecorder.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d, want %d", logoutRecorder.Code, http.StatusSeeOther)
	}

	afterLogout := getWithCookie(t, server, "/account", cookie.Value)
	if afterLogout.Code != http.StatusSeeOther {
		t.Errorf("account after logout status = %d, want %d", afterLogout.Code, http.StatusSeeOther)
	}
}

func TestSignInPageRendersMagicLinkForm(t *testing.T) {
	loader, err := templates.New()
	if err != nil {
		t.Fatal(err)
	}

	server := newTestServerWith(t, ServerConfig{
		AuthService:       &stubAuthService{},
		Users:             newStubUserStore(),
		PagesRenderer:     loader,
		CookieSecure:      true,
		SessionCookieName: secureSessionCookieName,
	})

	recorder := getWithCookie(t, server, "/sign-in", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `hx-post="/auth/magic-link"`) {
		t.Errorf("sign-in page does not post to the Stratus magic-link route")
	}
	if !strings.Contains(body, "Get sign-in link") {
		t.Errorf("sign-in page has no link request button")
	}
	if strings.Contains(body, `name="password"`) {
		t.Errorf("sign-in page still asks for a password")
	}
}

func TestMagicLinkEmailTemplateRenders(t *testing.T) {
	loader, err := templates.New()
	if err != nil {
		t.Fatal(err)
	}

	const link = "https://cloud.example.com/auth/magic-link/consume?token=abc123"
	body, err := loader.RenderEmail(templates.TemplateMagicLink, struct{ Link string }{Link: link})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, link) {
		t.Errorf("magic-link email does not contain the link: %s", body)
	}
}

func TestAccountPageRendersWithRealTemplates(t *testing.T) {
	loader, err := templates.New()
	if err != nil {
		t.Fatal(err)
	}

	users := newStubUserStore()
	users.users[testEmail] = database.UserAccount{Email: testEmail, Active: true}
	server := newTestServerWith(t, ServerConfig{
		AuthService:       &stubAuthService{identity: auth.Identity{Email: testEmail}},
		Users:             users,
		PagesRenderer:     loader,
		CookieSecure:      true,
		SessionCookieName: secureSessionCookieName,
	})

	recorder := getWithCookie(t, server, "/account", testToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Your account") {
		t.Errorf("account page body missing heading: %s", body)
	}
	if strings.Contains(body, "unverified") {
		t.Errorf("account page still mentions verification")
	}
	if strings.Contains(body, "Reset password") {
		t.Errorf("account page still offers a password reset")
	}
}

func TestRequireAuthRedirectsAnonymous(t *testing.T) {
	server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, nil)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"root", http.MethodGet, "/"},
		{"account page", http.MethodGet, "/account"},
		{"setActive", http.MethodPost, "/account/setActive"},
		{"logout all", http.MethodPost, "/account/logout-all"},
		{"delete", http.MethodPost, "/account/delete"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			recorder := httptest.NewRecorder()
			server.Routes().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
			}
			if got := recorder.Header().Get("Location"); got != "/sign-in" {
				t.Errorf("Location = %q, want /sign-in", got)
			}
		})
	}
}

func TestAccountPageRequiresSessionAndCreatesProfile(t *testing.T) {
	authService := &stubAuthService{identity: auth.Identity{Email: testEmail}}
	users := newStubUserStore()
	server := newTestServer(t, authService, users)

	recorder := getWithCookie(t, server, "/account", testToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "account.tmpl") {
		t.Errorf("body %q does not reference the account page", recorder.Body.String())
	}

	user, ok := users.users[testEmail]
	if !ok {
		t.Fatal("authenticated request did not create a user profile")
	}
	if !user.Active {
		t.Error("new profile should be active")
	}
}

func TestProfileCreationFailureIsInternalError(t *testing.T) {
	users := newStubUserStore()
	users.getErr = errors.New("database down")
	server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, users)

	recorder := getWithCookie(t, server, "/account", testToken)
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestSetActiveSwapsStatusFragment(t *testing.T) {
	tests := []struct {
		name      string
		active    string
		wantState bool
	}{
		{"activate", "true", true},
		{"deactivate", "false", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users := newStubUserStore()
			users.users[testEmail] = database.UserAccount{Email: testEmail, Active: !tc.wantState}
			server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, users)

			recorder := postAuthedForm(t, server, "/account/setActive", url.Values{"activeStatus": {tc.active}})

			if recorder.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := recorder.Header().Get("HX-Redirect"); got != "" {
				t.Errorf("setActive must not redirect; HX-Redirect = %q", got)
			}
			if !strings.Contains(recorder.Body.String(), "account_active_status") {
				t.Errorf("body %q does not reference account_active_status", recorder.Body.String())
			}
			if got := users.users[testEmail].Active; got != tc.wantState {
				t.Errorf("SetActive(%v) recorded, want %v", got, tc.wantState)
			}
		})
	}
}

func TestSetActiveRejectsBadValue(t *testing.T) {
	server := newTestServer(t, &stubAuthService{identity: auth.Identity{Email: testEmail}}, nil)

	recorder := postAuthedForm(t, server, "/account/setActive", url.Values{"activeStatus": {"other"}})

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "account_error") {
		t.Errorf("body %q does not reference account_error", recorder.Body.String())
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	authService := &stubAuthService{identity: auth.Identity{Email: testEmail}}
	server := newTestServer(t, authService, nil)

	recorder := postAuthedForm(t, server, "/logout", nil)
	cookie := sessionCookie(t, recorder)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if got := recorder.Header().Get("HX-Redirect"); got != "/sign-in" {
		t.Errorf("HX-Redirect = %q, want /sign-in", got)
	}
	if cookie.Value != "" {
		t.Errorf("cookie value = %q, want empty", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want a non-positive value", cookie.MaxAge)
	}
	if !cookie.HttpOnly || !cookie.Secure {
		t.Error("cleared cookie must keep HttpOnly and Secure attributes")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if len(authService.loggedOut) != 1 || authService.loggedOut[0] != testToken {
		t.Errorf("Logout calls = %v, want [%q]", authService.loggedOut, testToken)
	}
}

func TestLogoutAllRevokesEverySession(t *testing.T) {
	authService := &stubAuthService{identity: auth.Identity{Email: testEmail}}
	server := newTestServer(t, authService, nil)

	recorder := postAuthedForm(t, server, "/account/logout-all", nil)
	cookie := sessionCookie(t, recorder)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if got := recorder.Header().Get("HX-Redirect"); got != "/sign-in" {
		t.Errorf("HX-Redirect = %q, want /sign-in", got)
	}
	if cookie.Value != "" || cookie.MaxAge >= 0 {
		t.Errorf("logout-all should clear the session cookie, got value %q MaxAge %d", cookie.Value, cookie.MaxAge)
	}
	if len(authService.loggedOutAll) != 1 || authService.loggedOutAll[0] != testEmail {
		t.Errorf("LogoutAll calls = %v, want [%q]", authService.loggedOutAll, testEmail)
	}
}

func TestDeleteRemovesProfileAndClearsSessionCookie(t *testing.T) {
	authService := &stubAuthService{identity: auth.Identity{Email: testEmail}}
	users := newStubUserStore()
	users.users[testEmail] = database.UserAccount{Email: testEmail, Active: true}
	server := newTestServer(t, authService, users)

	recorder := postAuthedForm(t, server, "/account/delete", nil)
	cookie := sessionCookie(t, recorder)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if got := recorder.Header().Get("HX-Redirect"); got != "/sign-in" {
		t.Errorf("HX-Redirect = %q, want /sign-in", got)
	}
	if cookie.Value != "" || cookie.MaxAge >= 0 {
		t.Errorf("delete should clear the session cookie, got value %q MaxAge %d", cookie.Value, cookie.MaxAge)
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

	recorder := postAuthedForm(t, server, "/account/delete", nil)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if len(users.deleted) != 0 {
		t.Errorf("profile was deleted although sessions could not be revoked: %v", users.deleted)
	}
}

func TestRenderFailureIsInternalError(t *testing.T) {
	server := newTestServerWith(t, ServerConfig{
		AuthService:       &stubAuthService{},
		Users:             newStubUserStore(),
		PagesRenderer:     stubRenderer{err: errors.New("boom")},
		SessionCookieName: secureSessionCookieName,
	})

	recorder := getWithCookie(t, server, "/sign-in", "")
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestAuthErrorStatus(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"rate limited", auth.ErrRateLimited, http.StatusTooManyRequests},
		{"invalid email", auth.ErrInvalidEmail, http.StatusBadRequest},
		{"unauthenticated", auth.ErrUnauthenticated, http.StatusUnauthorized},
		{"invalid input", ErrInvalidInput, http.StatusBadRequest},
		{"unexpected", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := authErrorStatus(tc.err)
			if status != tc.status {
				t.Errorf("status = %d, want %d", status, tc.status)
			}
		})
	}
}
