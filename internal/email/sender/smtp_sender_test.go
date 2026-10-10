package sender

import (
	"testing"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
)

func TestNewSMTPRejectsBadFrom(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic for missing required config")
		}
	}()
	NewSMTPSender(
		SMTPConfig{
			ServerAuth: email.ServerAuth{
				Host:     "host",
				Port:     "465",
				Username: "username",
				Password: "secret",
			},
			FromAddress: "not-an-address",
			Timeout:     time.Second,
		})
}

func TestNewSMTPSenderPanicsOnMissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  SMTPConfig
	}{
		{"missing host", SMTPConfig{ServerAuth: email.ServerAuth{
			Port:     "465",
			Username: "username",
			Password: "secret",
		}, FromAddress: "noreply@example.com"}},
		{"missing port", SMTPConfig{ServerAuth: email.ServerAuth{
			Host:     "host",
			Username: "username",
			Password: "secret",
		}, FromAddress: "noreply@example.com"}},
		{"missing from address", SMTPConfig{ServerAuth: email.ServerAuth{
			Host:     "host",
			Port:     "465",
			Username: "username",
			Password: "secret",
		}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected a panic for missing required config")
				}
			}()
			_ = NewSMTPSender(tc.cfg)
		})
	}
}
