package email

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/mail"
	"strings"
	"testing"
	"time"
)

// Records the arguments of the last deliver call.
type mockMailer struct {
	from, host, port, username, password string
	recipients                           []string
	body                                 []byte
	err                                  error
	calls                                int
}

func (m *mockMailer) deliver(_ context.Context, params mailerDeliverParams) error {
	m.calls++
	m.from, m.host, m.port, m.username, m.password = params.from, params.host, params.port, params.username, params.password
	m.recipients, m.body = params.recipients, params.body
	return m.err
}

func newTestSender(t *testing.T, m *mockMailer) *SMTPSender {
	t.Helper()
	sender, err := NewSMTPSender(SMTPConfig{
		Host:        "smtp.example.com",
		Port:        "465",
		Username:    "user",
		Password:    "secret",
		FromAddress: "noreply@example.com",
		FromName:    "Cloud",
		Timeout:     time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	sender.mailer = m
	return sender
}

func TestSend(t *testing.T) {
	mock := &mockMailer{}
	sender := newTestSender(t, mock)

	err := sender.Send(context.Background(), Message{Subject: "Hi ☁", Body: "Hello cloud"}, []string{"a@example.com", "Bob <b@example.com>"})
	if err != nil {
		t.Fatal(err)
	}

	if mock.host != "smtp.example.com" || mock.username != "user" || mock.password != "secret" {
		t.Errorf("connection target/credentials = %q %q/%q", mock.host, mock.username, mock.password)
	}
	if mock.from != "noreply@example.com" {
		t.Errorf("from = %q", mock.from)
	}
	if strings.Join(mock.recipients, ",") != "a@example.com,b@example.com" {
		t.Errorf("recipients = %v, want bare addresses", mock.recipients)
	}

	parsed, err := mail.ReadMessage(strings.NewReader(string(mock.body)))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Header.Get("From") != `"Cloud" <noreply@example.com>` {
		t.Errorf("From header = %q", parsed.Header.Get("From"))
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil || subject != "Hi ☁" {
		t.Errorf("Subject = %q (%v)", subject, err)
	}
	if parsed.Header.Get("Message-ID") == "" || parsed.Header.Get("Date") == "" {
		t.Errorf("missing headers: %v", parsed.Header)
	}
	body, _ := io.ReadAll(parsed.Body)
	if strings.TrimSpace(string(body)) != "Hello cloud" {
		t.Errorf("body = %q", body)
	}
	// Recipients are blind-copied: they must not appear in any recipient header.
	if parsed.Header.Get("Cc") != "" || parsed.Header.Get("Bcc") != "" {
		t.Errorf("unexpected Cc/Bcc header: %v", parsed.Header)
	}
	if to := parsed.Header.Get("To"); strings.Contains(to, "a@example.com") || strings.Contains(to, "b@example.com") {
		t.Errorf("recipient leaked into To header: %q", to)
	}
	for _, addr := range []string{"a@example.com", "b@example.com"} {
		if strings.Contains(string(mock.body), addr) {
			t.Errorf("recipient %q leaked into message", addr)
		}
	}
}

func TestSendInvalidInput(t *testing.T) {
	// Fails if validation reaches the mailer.
	mock := &mockMailer{}

	tests := []struct {
		name       string
		msg        Message
		recipients []string
		want       error
	}{
		{"empty subject", Message{Body: "x"}, []string{"a@example.com"}, ErrInvalidMessage},
		{"subject injection", Message{Subject: "a\r\nBcc: x@example.com", Body: "x"}, []string{"a@example.com"}, ErrInvalidMessage},
		{"empty body", Message{Subject: "s"}, []string{"a@example.com"}, ErrInvalidMessage},
		{"no recipients", Message{Subject: "s", Body: "x"}, nil, ErrNoRecipients},
		{"bad recipient", Message{Subject: "s", Body: "x"}, []string{"nope"}, ErrInvalidRecipient},
		{"recipient injection", Message{Subject: "s", Body: "x"}, []string{"a@example.com\r\nBcc: x@example.com"}, ErrInvalidRecipient},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sender := newTestSender(t, mock)
			if err := sender.Send(context.Background(), tc.msg, tc.recipients); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
	if mock.calls != 0 {
		t.Errorf("mailer called %d times for invalid input", mock.calls)
	}
}

func TestSendPropagatesMailerError(t *testing.T) {
	mock := &mockMailer{err: ErrDelivery}
	sender := newTestSender(t, mock)

	err := sender.Send(context.Background(), Message{Subject: "s", Body: "x"}, []string{"a@example.com"})
	if !errors.Is(err, ErrDelivery) {
		t.Errorf("err = %v, want ErrDelivery", err)
	}
}
