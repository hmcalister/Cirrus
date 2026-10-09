package email

import (
	"context"
	"fmt"
	"time"
)

// SMTPConfig configures a SMTPSender.
type SMTPConfig struct {
	Host        string
	Port        string
	Username    string
	Password    string
	FromAddress string
	FromName    string
	Timeout     time.Duration
}

type SMTPSender struct {
	host        string
	port        string
	username    string
	password    string
	fromName    string
	fromAddress string
	timeout     time.Duration
	mailer      mailer
}

// Create a new SMTPSender from cfg. No connection is made until Send is called.
//
// Host, Port, and FromAddress are required and must be non-empty; a missing
// value is a programming error and panics.
func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if cfg.Host == "" {
		panic("email: SMTPConfig.Host must not be empty")
	}
	if cfg.Port == "" {
		panic("email: SMTPConfig.Port must not be empty")
	}
	if cfg.FromAddress == "" {
		panic("email: SMTPConfig.FromAddress must not be empty")
	}

	addr, err := parseAddress(cfg.FromAddress)
	if err != nil {
		return nil, fmt.Errorf("email: invalid from address %q: %w", cfg.FromAddress, err)
	}

	return &SMTPSender{
		host:        cfg.Host,
		port:        cfg.Port,
		username:    cfg.Username,
		password:    cfg.Password,
		fromName:    cfg.FromName,
		fromAddress: addr,
		timeout:     cfg.Timeout,
		mailer:      smtpMailer{},
	}, nil
}

// Send implements Sender.
//
// Invalid input is rejected before any network activity.
// A cancelled or expired context is reported as the context's error.
func (s *SMTPSender) Send(ctx context.Context, msg Message, recipients []string) error {
	if err := validateMessage(msg); err != nil {
		return err
	}
	addrs, err := validateRecipients(recipients)
	if err != nil {
		return err
	}
	body, err := buildMIME(s.fromName, s.fromAddress, msg)
	if err != nil {
		return err
	}

	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	return s.mailer.deliver(ctx, mailerDeliverParams{
		from:       s.fromAddress,
		host:       s.host,
		port:       s.port,
		username:   s.username,
		password:   s.password,
		recipients: addrs,
		body:       body,
	})
}
