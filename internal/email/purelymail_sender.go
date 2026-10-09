package email

import (
	"context"
	"fmt"
	"time"
)

// PurelyMailConfig configures a PurelyMailSender.
type PurelyMailConfig struct {
	Host        string
	Port        string
	Username    string
	Password    string
	FromAddress string
	FromName    string
	Timeout     time.Duration
}

type PurelyMailSender struct {
	host        string
	port        string
	username    string
	password    string
	fromName    string
	fromAddress string
	timeout     time.Duration
	mailer      mailer
}

// NewPurelyMail creates a sender from cfg. No connection is made until Send is called.
//
// Host, Port, and FromAddress are required and must be non-empty; a missing
// value is a programming error and panics.
func NewPurelyMail(cfg PurelyMailConfig) (*PurelyMailSender, error) {
	if cfg.Host == "" {
		panic("email: PurelyMailConfig.Host must not be empty")
	}
	if cfg.Port == "" {
		panic("email: PurelyMailConfig.Port must not be empty")
	}
	if cfg.FromAddress == "" {
		panic("email: PurelyMailConfig.FromAddress must not be empty")
	}

	addr, err := parseAddress(cfg.FromAddress)
	if err != nil {
		return nil, fmt.Errorf("email: invalid from address %q: %w", cfg.FromAddress, err)
	}

	return &PurelyMailSender{
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
func (s *PurelyMailSender) Send(ctx context.Context, msg Message, recipients []string) error {
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
