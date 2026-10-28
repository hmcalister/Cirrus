package main

import (
	"context"
	"log/slog"

	"github.com/hmcalister/cirrus/internal/config"
	"github.com/hmcalister/skysatchel/pkg/email/skysender"
	"github.com/hmcalister/skysatchel/pkg/email/skysender/ratelimit"
)

var _ skysender.Sender = StdoutSender{}

type StdoutSender struct{}

// Send implements [skysender.Sender].
func (s StdoutSender) Send(ctx context.Context, params skysender.SendParams) error {
	slog.Debug("sender invoked", "send_params", params)
	return nil
}

func constructSender(cfg config.ServerConfig) (*skysender.Sender, error) {
	var emailSender skysender.Sender
	if !cfg.Debug {
		emailSender = StdoutSender{}
	} else {
		baseSender, err := skysender.NewSMTPSender(skysender.SMTPConfig{
			Host:        cfg.Email.Host,
			Port:        cfg.Email.Port,
			Username:    cfg.Email.Username,
			Password:    cfg.Email.Password,
			FromAddress: cfg.Email.FromAddress,
			FromName:    cfg.Email.FromName,
			Timeout:     cfg.Email.Timeout,
		})
		if err != nil {
			slog.Error("error while creating new skysender smtp sender", "err", err)
			return nil, err
		}

		emailSender, err = ratelimit.NewRateLimitedSender(ratelimit.RateLimitedSenderConfig{
			Next:                 baseSender,
			AddressBurst:         cfg.EmailRateLimit.AddressBurst,
			AddressInterval:      cfg.EmailRateLimit.AddressInterval,
			GlobalBurst:          cfg.EmailRateLimit.GlobalBurst,
			GlobalInterval:       cfg.EmailRateLimit.GlobalInterval,
			SweepCadenceDuration: cfg.EmailRateLimit.SweepCadenceDuration,
		})
		if err != nil {
			slog.Error("error while creating new rate limited skysender", "err", err)
			return nil, err
		}
	}

	return &emailSender, nil
}
