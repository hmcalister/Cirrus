package sender

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
	"github.com/hmcalister/Cirrus/internal/email/mailer"
)

// Records the arguments of the last deliver call.
type mockMailer struct {
	from, host, port, username, password string
	recipients                           []string
	body                                 []byte
	err                                  error
	calls                                int
}

func (m *mockMailer) Deliver(_ context.Context, params mailer.MailerDeliverParams) error {
	m.calls++
	m.host, m.port, m.username, m.password = params.ServerAuth.Host, params.ServerAuth.Port, params.ServerAuth.Username, params.ServerAuth.Password
	m.from, m.recipients, m.body = params.From, params.Recipients, params.Body
	return m.err
}

func newTestSender(t *testing.T, m *mockMailer) *SMTPSender {
	t.Helper()
	sender := NewSMTPSender(SMTPConfig{
		ServerAuth: email.ServerAuth{
			Host:     "smtp.example.com",
			Port:     "465",
			Username: "user",
			Password: "secret",
		},
		FromAddress: "noreply@example.com",
		FromName:    "Cloud",
		Timeout:     time.Second,
	})
	sender.mailer = m
	return sender
}

func TestSend(t *testing.T) {
	mock := &mockMailer{}
	sender := newTestSender(t, mock)

	err := sender.Send(context.Background(), email.Message{Subject: "Hello, World!", Body: "Hello cloud"}, []string{"recipient1@example.com", "Bob <recipient2@example.com>"})
	if err != nil {
		t.Fatal(err)
	}

	if mock.host != "smtp.example.com" || mock.username != "user" || mock.password != "secret" {
		t.Logf("%+v", *mock)
		t.Errorf("connection target/credentials = %q %q/%q", mock.host, mock.username, mock.password)
	}
	if mock.from != "noreply@example.com" {
		t.Errorf("from = %q", mock.from)
	}
	if strings.Join(mock.recipients, ",") != "recipient1@example.com,recipient2@example.com" {
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
	if err != nil || subject != "Hello, World!" {
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
	if to := parsed.Header.Get("To"); strings.Contains(to, "recipient1@example.com") || strings.Contains(to, "recipient2@example.com") {
		t.Errorf("recipient leaked into To header: %q", to)
	}
	for _, addr := range []string{"recipient1@example.com", "recipient2@example.com"} {
		if strings.Contains(string(mock.body), addr) {
			t.Errorf("recipient %q leaked into message", addr)
		}
	}
}

func TestSendWithAttachments(t *testing.T) {
	mock := &mockMailer{}
	sender := newTestSender(t, mock)

	msg := email.Message{
		Subject: "Report",
		Body:    "See attached.",
		Attachments: []email.Attachment{
			{Filename: "hello.txt", Data: []byte("hello world")},
			{Filename: "data.bin", ContentType: "application/x-custom", Data: []byte{0x00, 0x01, 0x02, 0xff}},
		},
	}
	if err := sender.Send(context.Background(), msg, []string{"recipient1@example.com"}); err != nil {
		t.Fatal(err)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(mock.body))
	if err != nil {
		t.Fatal(err)
	}

	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	if mediaType != "multipart/mixed" {
		t.Fatalf("Content-Type = %q, want multipart/mixed", mediaType)
	}

	mr := multipart.NewReader(parsed.Body, params["boundary"])

	// First part is the text body.
	textPart, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if ct := textPart.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("first part Content-Type = %q, want text/plain", ct)
	}
	textBody, _ := io.ReadAll(textPart)
	if strings.TrimSpace(string(textBody)) != "See attached." {
		t.Errorf("text body = %q", textBody)
	}

	want := []struct {
		filename string
		ct       string
		data     []byte
	}{
		{"hello.txt", "text/plain; charset=utf-8", []byte("hello world")},
		{"data.bin", "application/x-custom", []byte{0x00, 0x01, 0x02, 0xff}},
	}
	for i, w := range want {
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("attachment %d: %v", i, err)
		}
		if part.FileName() != w.filename {
			t.Errorf("attachment %d filename = %q, want %q", i, part.FileName(), w.filename)
		}
		if got := part.Header.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
			t.Errorf("attachment %d Content-Disposition = %q", i, got)
		}
		if got := part.Header.Get("Content-Transfer-Encoding"); got != "base64" {
			t.Errorf("attachment %d encoding = %q, want base64", i, got)
		}
		if got := part.Header.Get("Content-Type"); got != w.ct {
			t.Errorf("attachment %d Content-Type = %q, want %q", i, got, w.ct)
		}
		encoded, _ := io.ReadAll(part)
		data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(encoded)))
		if err != nil {
			t.Fatalf("attachment %d decode: %v", i, err)
		}
		if !bytes.Equal(data, w.data) {
			t.Errorf("attachment %d data = %q, want %q", i, data, w.data)
		}
	}
}

func TestSendInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		msg        email.Message
		recipients []string
		want       error
	}{
		{"empty subject", email.Message{Body: "x"}, []string{"recipient1@example.com"}, email.ErrInvalidMessage},
		{"subject injection", email.Message{Subject: "a\r\nBcc: x@example.com", Body: "x"}, []string{"recipient1@example.com"}, email.ErrInvalidMessage},
		{"empty body", email.Message{Subject: "s"}, []string{"recipient1@example.com"}, email.ErrInvalidMessage},
		{"empty attachment filename", email.Message{Subject: "s", Body: "x", Attachments: []email.Attachment{{Data: []byte("d")}}}, []string{"recipient1@example.com"}, email.ErrInvalidMessage},
		{"attachment filename injection", email.Message{Subject: "s", Body: "x", Attachments: []email.Attachment{{Filename: "a\r\nX: y", Data: []byte("d")}}}, []string{"recipient1@example.com"}, email.ErrInvalidMessage},
		{"attachment content type injection", email.Message{Subject: "s", Body: "x", Attachments: []email.Attachment{{Filename: "a.txt", ContentType: "text/plain\r\nX: y", Data: []byte("d")}}}, []string{"recipient1@example.com"}, email.ErrInvalidMessage},
		{"empty attachment data", email.Message{Subject: "s", Body: "x", Attachments: []email.Attachment{{Filename: "a.txt"}}}, []string{"recipient1@example.com"}, email.ErrInvalidMessage},
		{"no recipients", email.Message{Subject: "s", Body: "x"}, nil, email.ErrNoRecipients},
		{"bad recipient", email.Message{Subject: "s", Body: "x"}, []string{"nope"}, email.ErrInvalidRecipient},
		{"recipient injection", email.Message{Subject: "s", Body: "x"}, []string{"recipient1@example.com\r\nBcc: x@example.com"}, email.ErrInvalidRecipient},
	}
	for _, tc := range tests {
		mock := &mockMailer{}
		t.Run(tc.name, func(t *testing.T) {
			sender := newTestSender(t, mock)
			if err := sender.Send(context.Background(), tc.msg, tc.recipients); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
		if mock.calls != 0 {
			t.Errorf("mailer called %d times for invalid input", mock.calls)
		}
	}
}

func TestSendPropagatesMailerError(t *testing.T) {
	mock := &mockMailer{err: email.ErrDelivery}
	sender := newTestSender(t, mock)

	err := sender.Send(context.Background(), email.Message{Subject: "s", Body: "x"}, []string{"recipient1@example.com"})
	if !errors.Is(err, email.ErrDelivery) {
		t.Errorf("err = %v, want ErrDelivery", err)
	}
}
