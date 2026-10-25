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

	"github.com/hmcalister/cirrus/internal/authentication"
)

const (
	testEmail    = "user@example.com"
	testPassword = "hunter2"
	testToken    = "raw-session-token"
)

// Stub of the authService seam. Records the calls it receives and returns the
// behaviour configured by the test.
type stubAuthService struct {
	sessionEmail    string
	sessionOK       bool
	active          bool
	verified        bool
	err             error
	signInToken     string
	signInErr       error
	verifyResult    bool
	verifyErr       error
	lastEmail       string
	lastPassword    string
	lastToken       string
	lastNewPassword string

	registeredEmails []string
	activeEmails     map[string]bool
	deleted          []string
}

func (s *stubAuthService) Register(_ context.Context, userEmail, password string) error {
	s.lastEmail, s.lastPassword = userEmail, password
	if s.err != nil {
		return s.err
	}
	s.registeredEmails = append(s.registeredEmails, userEmail)
	return nil
}

func (s *stubAuthService) Verify(_ context.Context, rawToken string) (bool, error) {
	s.lastToken = rawToken
	return s.verifyResult, s.verifyErr
}

func (s *stubAuthService) SignIn(_ context.Context, userEmail, password string) (string, error) {
	s.lastEmail, s.lastPassword = userEmail, password
	return s.signInToken, s.signInErr
}

func (s *stubAuthService) ResendVerification(_ context.Context, userEmail string) error {
	s.lastEmail = userEmail
	return s.err
}

func (s *stubAuthService) RequestPasswordReset(_ context.Context, userEmail string) error {
	s.lastEmail = userEmail
	return s.err
}

func (s *stubAuthService) ConfirmPasswordReset(_ context.Context, rawToken, newPassword string) error {
	s.lastToken, s.lastNewPassword = rawToken, newPassword
	return s.err
}

func (s *stubAuthService) SetAccountActive(_ context.Context, userEmail string, active bool) error {
	if s.activeEmails == nil {
		s.activeEmails = make(map[string]bool)
	}
	s.activeEmails[userEmail] = active
	return s.err
}

func (s *stubAuthService) DeleteAccount(_ context.Context, userEmail string) error {
	s.deleted = append(s.deleted, userEmail)
	return s.err
}

func (s *stubAuthService) SessionEmail(_ context.Context, _ string) (string, bool) {
	return s.sessionEmail, s.sessionOK
}

func (s *stubAuthService) AccountActive(_ context.Context, _ string) (bool, error) {
	return s.active, nil
}

func (s *stubAuthService) AccountVerified(_ context.Context, _ string) (bool, error) {
	return s.verified, nil
}

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

var _ authService = (*stubAuthService)(nil)

func newTestServer(t *testing.T, auth *stubAuthService) *Server {
	t.Helper()

	server, err := NewServer(ServerConfig{
		AuthService:   auth,
		PagesRenderer: stubRenderer{},
		CookieSecure:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

// Posts url.Values to path through the server's routes.
func postForm(t *testing.T, server *Server, path string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)
	return recorder
}

func getWithCookie(t *testing.T, server *Server, path, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		request.AddCookie(&http.Cookie{Name: authenticationCookieName, Value: cookie})
	}
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)
	return recorder
}

func sessionCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == authenticationCookieName {
			return cookie
		}
	}
	t.Fatalf("response has no %q cookie; headers: %v", authenticationCookieName, recorder.Header())
	return nil
}

func TestNewServerValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  ServerConfig
	}{
		{"nil auth service", ServerConfig{PagesRenderer: stubRenderer{}}},
		{"nil pages renderer", ServerConfig{AuthService: &stubAuthService{}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewServer(tc.cfg); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestPostErrorsRenderFragmentWithStatus(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		form     url.Values
		authErr  error
		template string
		status   int
	}{
		{
			name:     "bad sign in",
			path:     "/sign-in",
			form:     url.Values{"email": {testEmail}, "password": {testPassword}},
			authErr:  authentication.ErrBadSignIn,
			template: "sign_in_error",
			status:   http.StatusUnauthorized,
		},
		{
			name:     "register email taken",
			path:     "/register",
			form:     url.Values{"email": {testEmail}, "password": {testPassword}},
			authErr:  fmt.Errorf("%w: %s", authentication.ErrEmailTaken, testEmail),
			template: "register_error",
			status:   http.StatusConflict,
		},
		{
			name:     "register invalid email",
			path:     "/register",
			form:     url.Values{"email": {"nope"}, "password": {testPassword}},
			authErr:  authentication.ErrInvalidEmail,
			template: "register_error",
			status:   http.StatusBadRequest,
		},
		{
			name:     "reset bad token",
			path:     "/reset",
			form:     url.Values{"token": {testToken}, "password": {testPassword}},
			authErr:  authentication.ErrBadToken,
			template: "reset_error",
			status:   http.StatusBadRequest,
		},
		{
			name:     "reset invalid password",
			path:     "/reset",
			form:     url.Values{"token": {testToken}, "password": {""}},
			authErr:  authentication.ErrInvalidPassword,
			template: "reset_error",
			status:   http.StatusBadRequest,
		},
		{
			name:     "account setActive unexpected error",
			path:     "/account/setActive",
			form:     url.Values{"token": {testToken}, "activeStatus": {"false"}},
			authErr:  errors.New("database down"),
			template: "account_error",
			status:   http.StatusInternalServerError,
		},
		{
			name:     "account setActive bas request",
			path:     "/account/setActive",
			form:     url.Values{"token": {testToken}, "activeStatus": {"other"}},
			authErr:  nil,
			template: "account_error",
			status:   http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			auth := &stubAuthService{err: tc.authErr, signInErr: tc.authErr, sessionEmail: testEmail, sessionOK: true}
			server := newTestServer(t, auth)

			var recorder *httptest.ResponseRecorder
			if strings.HasPrefix(tc.path, "/account") {
				recorder = postAuthedForm(t, server, tc.path, tc.form)
			} else {
				recorder = postForm(t, server, tc.path, tc.form)
			}

			if recorder.Code != tc.status {
				t.Errorf("status = %d, want %d", recorder.Code, tc.status)
			}
			if !strings.Contains(recorder.Body.String(), tc.template) {
				t.Errorf("body %q does not reference template %q", recorder.Body.String(), tc.template)
			}
			if recorder.Header().Get("HX-Redirect") != "" {
				t.Errorf("error response must not redirect; HX-Redirect = %q", recorder.Header().Get("HX-Redirect"))
			}
			if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want text/html", got)
			}
		})
	}
}

func TestPostSuccessRedirects(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		form     url.Values
		auth     *stubAuthService
		location string
	}{
		{
			name:     "sign in",
			path:     "/sign-in",
			form:     url.Values{"email": {testEmail}, "password": {testPassword}},
			auth:     &stubAuthService{signInToken: testToken},
			location: "/account",
		},
		{
			name:     "register",
			path:     "/register",
			form:     url.Values{"email": {testEmail}, "password": {testPassword}},
			auth:     &stubAuthService{},
			location: "/sign-in",
		},
		{
			name:     "logout",
			path:     "/logout",
			form:     nil,
			auth:     &stubAuthService{},
			location: "/sign-in",
		},
		{
			name:     "reset",
			path:     "/reset",
			form:     url.Values{"token": {testToken}, "password": {testPassword}},
			auth:     &stubAuthService{},
			location: "/sign-in",
		},
		{
			name:     "account delete",
			path:     "/account/delete",
			form:     nil,
			auth:     &stubAuthService{sessionEmail: testEmail, sessionOK: true},
			location: "/sign-in",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newTestServer(t, tc.auth)

			var recorder *httptest.ResponseRecorder
			if strings.HasPrefix(tc.path, "/account") {
				recorder = postAuthedForm(t, server, tc.path, tc.form)
			} else {
				recorder = postForm(t, server, tc.path, tc.form)
			}

			if recorder.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
			}
			if got := recorder.Header().Get("HX-Redirect"); got != tc.location {
				t.Errorf("HX-Redirect = %q, want %q", got, tc.location)
			}
		})
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
			auth := &stubAuthService{sessionEmail: testEmail, sessionOK: true}
			server := newTestServer(t, auth)

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
			if got := auth.activeEmails[testEmail]; got != tc.wantState {
				t.Errorf("SetAccountActive(%v) recorded, want %v", got, tc.wantState)
			}
		})
	}
}

func TestSignInSetsSessionCookie(t *testing.T) {
	auth := &stubAuthService{signInToken: testToken}
	server := newTestServer(t, auth)

	recorder := postForm(t, server, "/sign-in", url.Values{"email": {testEmail}, "password": {testPassword}})
	cookie := sessionCookie(t, recorder)

	if cookie.Value != testToken {
		t.Errorf("cookie value = %q, want %q", cookie.Value, testToken)
	}
	if !cookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if !cookie.Secure {
		t.Error("session cookie must be Secure")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	server := newTestServer(t, &stubAuthService{})

	recorder := postForm(t, server, "/logout", nil)
	cookie := sessionCookie(t, recorder)

	if cookie.Value != "" {
		t.Errorf("cookie value = %q, want empty", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want a non-positive value", cookie.MaxAge)
	}
	if !cookie.HttpOnly || !cookie.Secure {
		t.Error("cleared cookie must keep HttpOnly and Secure attributes")
	}
}

func TestDeleteClearsSessionCookie(t *testing.T) {
	auth := &stubAuthService{sessionEmail: testEmail, sessionOK: true}
	server := newTestServer(t, auth)

	recorder := postAuthedForm(t, server, "/account/delete", nil)
	cookie := sessionCookie(t, recorder)

	if cookie.Value != "" || cookie.MaxAge >= 0 {
		t.Errorf("delete should clear the session cookie, got value %q MaxAge %d", cookie.Value, cookie.MaxAge)
	}
}

func TestRequireAuthRedirectsAnonymous(t *testing.T) {
	server := newTestServer(t, &stubAuthService{sessionOK: false})

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"account page", http.MethodGet, "/account"},
		{"setActive", http.MethodPost, "/account/setActive"},
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

func TestAccountPageRequiresSession(t *testing.T) {
	server := newTestServer(t, &stubAuthService{sessionEmail: testEmail, sessionOK: true, active: true})

	recorder := getWithCookie(t, server, "/account", testToken)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "account.tmpl") {
		t.Errorf("body %q does not reference the account page", recorder.Body.String())
	}
}

func TestPasswordResetDoesNotLeakAccountExistence(t *testing.T) {
	// A known account and an unknown account both render 200 with the same
	// status fragment, so the endpoint cannot be used to enumerate accounts.
	known := newTestServer(t, &stubAuthService{})
	unknown := newTestServer(t, &stubAuthService{})

	knownRecorder := postForm(t, known, "/password/reset", url.Values{"email": {"known@example.com"}})
	unknownRecorder := postForm(t, unknown, "/password/reset", url.Values{"email": {"unknown@example.com"}})

	if knownRecorder.Code != http.StatusOK || unknownRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d and %d, want both 200", knownRecorder.Code, unknownRecorder.Code)
	}
	if knownRecorder.Body.String() != unknownRecorder.Body.String() {
		t.Errorf("bodies differ:\n known: %s\n unknown: %s", knownRecorder.Body.String(), unknownRecorder.Body.String())
	}
	if knownRecorder.Header().Get("HX-Redirect") != "" {
		t.Error("status response must not redirect")
	}
}

func TestVerifyResendStatusDoesNotLeakExistence(t *testing.T) {
	known := newTestServer(t, &stubAuthService{})
	unknown := newTestServer(t, &stubAuthService{})

	knownRecorder := postForm(t, known, "/verify/resend", url.Values{"email": {"known@example.com"}})
	unknownRecorder := postForm(t, unknown, "/verify/resend", url.Values{"email": {"unknown@example.com"}})

	if knownRecorder.Code != http.StatusOK || unknownRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d and %d, want both 200", knownRecorder.Code, unknownRecorder.Code)
	}
	if knownRecorder.Body.String() != unknownRecorder.Body.String() {
		t.Errorf("bodies differ:\n known: %s\n unknown: %s", knownRecorder.Body.String(), unknownRecorder.Body.String())
	}
}

func TestVerifyRedirectsToSignIn(t *testing.T) {
	server := newTestServer(t, &stubAuthService{verifyResult: true})

	request := httptest.NewRequest(http.MethodGet, "/verify?token="+testToken, nil)
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if got := recorder.Header().Get("Location"); got != "/sign-in" {
		t.Errorf("Location = %q, want /sign-in", got)
	}
}

func TestVerifyBadTokenShowsError(t *testing.T) {
	server := newTestServer(t, &stubAuthService{verifyErr: authentication.ErrBadToken})

	request := httptest.NewRequest(http.MethodGet, "/verify?token=bad", nil)
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "sign_in_error") {
		t.Errorf("body %q does not reference sign_in_error", recorder.Body.String())
	}
}

func TestRenderFailureIsInternalError(t *testing.T) {
	server, err := NewServer(ServerConfig{
		AuthService:   &stubAuthService{},
		PagesRenderer: stubRenderer{err: errors.New("boom")},
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := postForm(t, server, "/password/reset", url.Values{"email": {testEmail}})
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

// postAuthedForm issues a POST carrying a session cookie so the request passes
// RequireAuth.
func postAuthedForm(t *testing.T, server *Server, path string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()

	var body *strings.Reader
	if values == nil {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader(values.Encode())
	}

	request := httptest.NewRequest(http.MethodPost, path, body)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: authenticationCookieName, Value: testToken})
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)
	return recorder
}
