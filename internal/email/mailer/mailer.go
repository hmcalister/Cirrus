package mailer

import (
	"context"
	"crypto/tls"

	"github.com/hmcalister/Cirrus/internal/email"
)

// tlsConfig returns the TLS configuration used for every connection.
func tlsConfig(host string) *tls.Config {
	return &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}
}

// mailer delivers a rendered message.
// This interface handles network connections and hence can be mocked.
type Mailer interface {
	Deliver(ctx context.Context, params MailerDeliverParams) error
}

type MailerDeliverParams struct {
	ServerAuth email.ServerAuth
	From       string
	Recipients []string
	Body       []byte
}
