package email

import (
	"testing"
	"time"
)

func TestNewSMTPRejectsBadFrom(t *testing.T) {
	if _, err := NewSMTPSender(SMTPConfig{Host: "host", Port: "465", Username: "u", Password: "p", FromAddress: "not-an-address", Timeout: time.Second}); err == nil {
		t.Error("expected an error for an invalid from address")
	}
}

func TestNewSMTPSenderPanicsOnMissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  SMTPConfig
	}{
		{"missing host", SMTPConfig{Port: "465", FromAddress: "noreply@example.com"}},
		{"missing port", SMTPConfig{Host: "host", FromAddress: "noreply@example.com"}},
		{"missing from address", SMTPConfig{Host: "host", Port: "465"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected a panic for missing required config")
				}
			}()
			_, _ = NewSMTPSender(tc.cfg)
		})
	}
}
