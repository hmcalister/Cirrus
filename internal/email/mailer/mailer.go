package mailer

import (
	"context"
	"crypto/tls"
	"errors"

	"github.com/hmcalister/Cirrus/internal/email"
)

// tlsConfig returns the TLS configuration used for every connection.
func tlsConfig(host string) *tls.Config {
	return &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}
}

type ServerAuth struct {
	host     string
	port     string
	username string
	password string
}

func NewServerAuth(host, port, username, password string) (ServerAuth, error) {
	if host == "" {
		return ServerAuth{}, errors.New("mailer: ServerAuth.Host must not be empty")
	}
	if port == "" {
		return ServerAuth{}, errors.New("mailer: ServerAuth.Port must not be empty")
	}
	if username == "" {
		return ServerAuth{}, errors.New("mailer: ServerAuth.username must not be empty")
	}
	if password == "" {
		return ServerAuth{}, errors.New("mailer: ServerAuth.password must not be empty")
	}

	return ServerAuth{
		host:     host,
		port:     port,
		username: username,
		password: password,
	}, nil
}

// mailer delivers a rendered message.
// This interface handles network connections and hence can be mocked.
type Mailer interface {
	Deliver(ctx context.Context, params MailerDeliverParams) error
}

type MailerDeliverParams struct {
	ServerAuth
	// Get using email.ValidateRecipients and mailer.EnvelopeRecipients
	AllRecipients []string
	From          string
	Body          []byte
}

// Return every recipient as a single, de-duplicated list. For SMTP envelope (RCPT TO).
//
// Addresses are assumed to be validated; any that fail to parse are skipped.
func EnvelopeRecipients(r email.Recipients) []string {
	total := len(r.ToRecipients) + len(r.CCRecipients) + len(r.BCCRecipients)
	if total == 0 {
		return nil
	}

	seen := make(map[string]struct{}, total)
	addrs := make([]string, 0, total)
	for _, group := range [][]string{r.ToRecipients, r.CCRecipients, r.BCCRecipients} {
		for _, raw := range group {
			addr, err := email.ParseAddress(raw)
			if err != nil {
				continue
			}
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			addrs = append(addrs, addr)
		}
	}

	return addrs
}
