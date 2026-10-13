package sender

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
	"github.com/hmcalister/Cirrus/internal/email/cirrusmime"
	"github.com/hmcalister/Cirrus/internal/email/mailer"
)

var _ Sender = SMTPSender{}

type SMTPConfig struct {
	mailer.ServerAuth
	FromAddress string
	FromName    string
	Timeout     time.Duration
}

type SMTPSender struct {
	mailer.ServerAuth
	fromAddress mail.Address
	timeout     time.Duration
	mailer      mailer.Mailer
}

// Create a new SMTPSender from cfg. No connection is made until Send is called.
//
// Host, Port, and FromAddress are required and must be non-empty.
func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if cfg.FromAddress == "" {
		return nil, errors.New("email: SMTPConfig.FromAddress must not be empty")
	}

	addr, err := email.ParseAddress(cfg.FromAddress)
	if err != nil {
		panic(fmt.Sprintf("email: invalid from address %q: %v", cfg.FromAddress, err))
	}

	return &SMTPSender{
		ServerAuth: cfg.ServerAuth,
		fromAddress: mail.Address{
			Name:    cfg.FromName,
			Address: addr,
		},
		timeout: cfg.Timeout,
		mailer:  mailer.SMTPMailer{},
	}, nil
}

// Send implements Sender.
//
// Invalid input is rejected before any network activity.
// A cancelled or expired context is reported as the context's error.
func (s SMTPSender) Send(ctx context.Context, sendParams SendParams) error {
	err := email.ValidateMessage(sendParams.Message)
	if err != nil {
		return err
	}

	err = email.ValidateRecipients(sendParams.Recipients)
	if err != nil {
		return err
	}

	body, err := cirrusmime.BuildMIMEMessage(s.fromAddress, sendParams.Recipients, sendParams.Message)
	if err != nil {
		return err
	}

	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	return s.mailer.Deliver(ctx, mailer.MailerDeliverParams{
		ServerAuth:    s.ServerAuth,
		From:          s.fromAddress.Address,
		AllRecipients: mailer.EnvelopeRecipients(sendParams.Recipients),
		Body:          body,
	})
}
