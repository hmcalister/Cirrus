package sender

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
	"github.com/hmcalister/Cirrus/internal/email/cirrusmime"
	"github.com/hmcalister/Cirrus/internal/email/mailer"
)

// SMTPConfig configures a SMTPSender.
type SMTPConfig struct {
	ServerAuth  email.ServerAuth
	FromAddress string
	FromName    string
	Timeout     time.Duration
}

type SMTPSender struct {
	serverAuth  email.ServerAuth
	fromAddress mail.Address
	timeout     time.Duration
	mailer      mailer.Mailer
}

// Create a new SMTPSender from cfg. No connection is made until Send is called.
//
// Host, Port, and FromAddress are required and must be non-empty; a missing
// value is a programming error and panics.
func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	if cfg.ServerAuth.Host == "" {
		panic("email: SMTPConfig.Host must not be empty")
	}
	if cfg.ServerAuth.Port == "" {
		panic("email: SMTPConfig.Port must not be empty")
	}
	if cfg.FromAddress == "" {
		panic("email: SMTPConfig.FromAddress must not be empty")
	}

	addr, err := email.ParseAddress(cfg.FromAddress)
	if err != nil {
		panic(fmt.Sprintf("email: invalid from address %q: %v", cfg.FromAddress, err))
	}

	return &SMTPSender{
		serverAuth: cfg.ServerAuth,
		fromAddress: mail.Address{
			Name:    cfg.FromName,
			Address: addr,
		},
		timeout: cfg.Timeout,
		mailer:  mailer.SMTPMailer{},
	}
}

// Send implements Sender.
//
// Invalid input is rejected before any network activity.
// A cancelled or expired context is reported as the context's error.
func (s *SMTPSender) Send(ctx context.Context, msg email.Message, recipients []string) error {
	if err := email.ValidateMessage(msg); err != nil {
		return err
	}
	addrs, err := email.ValidateRecipients(recipients)
	if err != nil {
		return err
	}
	body, err := cirrusmime.BuildMIMEMessage(s.fromAddress, msg)
	if err != nil {
		return err
	}

	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	return s.mailer.Deliver(ctx, mailer.MailerDeliverParams{
		ServerAuth: s.serverAuth,
		From:       s.fromAddress.Address,
		Recipients: addrs,
		Body:       body,
	})
}
