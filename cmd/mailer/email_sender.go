package main

import (
	"context"
	"log/slog"

	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
)

var _ skysender.Sender = StdoutSender{}

// Logs the message instead of sending it, so a debug run does not email every active user.
type StdoutSender struct{}

func (StdoutSender) Send(_ context.Context, params skysender.SendParams) error {
	slog.Debug("sender invoked", "send_params", params)
	return nil
}

// Returns the sender the daily job sends through, which only logs in debug mode.
func constructSender(cfg config.MailerConfig) (skysender.Sender, error) {
	if cfg.Debug {
		return StdoutSender{}, nil
	}

	sender, err := skysender.NewSMTPSender(skysender.SMTPConfig{
		Host:        cfg.Email.Host,
		Port:        cfg.Email.Port,
		Username:    cfg.Email.Username,
		Password:    cfg.Email.Password,
		FromAddress: cfg.Email.FromAddress,
		FromName:    cfg.Email.FromName,
		Timeout:     cfg.Email.Timeout,
	})
	if err != nil {
		return nil, err
	}

	return sender, nil
}
