package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
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
type mailer interface {
	deliver(ctx context.Context, params mailerDeliverParams) error
}

type mailerDeliverParams struct {
	from       string
	host       string
	port       string
	username   string
	password   string
	recipients []string
	body       []byte
}

// smtpMailer talks SMTP over implicit TLS.
type smtpMailer struct{}

func (smtpMailer) deliver(ctx context.Context, params mailerDeliverParams) error {
	conn, err := (&tls.Dialer{Config: tlsConfig(params.host)}).DialContext(ctx, "tcp", net.JoinHostPort(params.host, params.port))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConnection, err)
	}
	defer func() { _ = conn.Close() }()

	// net/smtp has no context support, so closing the connection unblocks its I/O.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	client, err := smtp.NewClient(conn, params.host)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConnection, err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Auth(smtp.PlainAuth("", params.username, params.password, params.host)); err != nil {
		return fmt.Errorf("%w: %w", ErrAuthentication, err)
	}
	if err := client.Mail(params.from); err != nil {
		return wrapSMTPError(err)
	}

	// RCPT TO is attempted for every recipient so that a single rejected address
	// does not prevent delivery to the others. Accepted recipients continue; any
	// rejection is collected and reported after the send attempt.
	var accepted []string
	var recipientErrs []error
	for _, rcpt := range params.recipients {
		if err := client.Rcpt(rcpt); err != nil {
			recipientErrs = append(recipientErrs, &RecipientError{Recipient: rcpt, Err: wrapSMTPError(err)})
			continue
		}
		accepted = append(accepted, rcpt)
	}

	if len(accepted) == 0 {
		// Nothing to deliver
		err := client.Quit()
		joinedRecipientErrs := errors.Join(recipientErrs...)
		return errors.Join(err, joinedRecipientErrs)
	}

	w, err := client.Data()
	if err != nil {
		return errors.Join(append(recipientErrs, wrapSMTPError(err))...)
	}
	if _, err := w.Write(params.body); err != nil {
		return errors.Join(append(recipientErrs, wrapSMTPError(err))...)
	}
	if err := w.Close(); err != nil {
		return errors.Join(append(recipientErrs, wrapSMTPError(err))...)
	}

	// The message is already accepted, so a failing QUIT is irrelevant.
	err = client.Quit()
	if len(recipientErrs) > 0 || err != nil {
		joinedRecipientErrs := errors.Join(recipientErrs...)
		return errors.Join(err, joinedRecipientErrs)
	}

	return nil
}
