package mailer

import (
	"context"
	"crypto/tls"
	"errors"
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
	From       string
	Recipients []string
	Body       []byte
}
