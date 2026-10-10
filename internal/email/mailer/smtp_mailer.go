package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"

	"github.com/hmcalister/Cirrus/internal/email"
)

var _ Mailer = SMTPMailer{}

// smtpMailer talks SMTP over implicit TLS.
type SMTPMailer struct{}

func (SMTPMailer) Deliver(ctx context.Context, params MailerDeliverParams) error {
	conn, err := (&tls.Dialer{Config: tlsConfig(params.ServerAuth.Host)}).DialContext(ctx, "tcp", net.JoinHostPort(params.ServerAuth.Host, params.ServerAuth.Port))
	if err != nil {
		return fmt.Errorf("%w: %w", email.ErrConnection, err)
	}
	defer func() { _ = conn.Close() }()

	// net/smtp has no context support, so closing the connection unblocks its I/O.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	client, err := smtp.NewClient(conn, params.ServerAuth.Host)
	if err != nil {
		return fmt.Errorf("%w: %w", email.ErrConnection, err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Auth(smtp.PlainAuth("", params.ServerAuth.Username, params.ServerAuth.Password, params.ServerAuth.Host)); err != nil {
		return fmt.Errorf("%w: %w", email.ErrAuthentication, err)
	}
	if err := client.Mail(params.From); err != nil {
		return email.WrapSMTPError(err)
	}

	// RCPT TO is attempted for every recipient so that a single rejected address
	// does not prevent delivery to the others. Accepted recipients continue; any
	// rejection is collected and reported after the send attempt.
	var accepted []string
	var recipientErrs []error
	for _, rcpt := range params.Recipients {
		if err := client.Rcpt(rcpt); err != nil {
			recipientErrs = append(recipientErrs, &email.RecipientError{Recipient: rcpt, Err: email.WrapSMTPError(err)})
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
		return errors.Join(append(recipientErrs, email.WrapSMTPError(err))...)
	}
	if _, err := w.Write(params.Body); err != nil {
		return errors.Join(append(recipientErrs, email.WrapSMTPError(err))...)
	}
	if err := w.Close(); err != nil {
		return errors.Join(append(recipientErrs, email.WrapSMTPError(err))...)
	}

	// The message is already accepted, so a failing QUIT is irrelevant.
	err = client.Quit()
	if len(recipientErrs) > 0 || err != nil {
		joinedRecipientErrs := errors.Join(recipientErrs...)
		return errors.Join(err, joinedRecipientErrs)
	}

	return nil
}
