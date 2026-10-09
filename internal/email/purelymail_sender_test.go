package email

import (
	"testing"
	"time"
)

func TestNewPurelyMailRejectsBadFrom(t *testing.T) {
	if _, err := NewPurelyMail(PurelyMailConfig{Host: "host", Port: "465", Username: "u", Password: "p", FromAddress: "not-an-address", Timeout: time.Second}); err == nil {
		t.Error("expected an error for an invalid from address")
	}
}

func TestNewPurelyMailPanicsOnMissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  PurelyMailConfig
	}{
		{"missing host", PurelyMailConfig{Port: "465", FromAddress: "noreply@example.com"}},
		{"missing port", PurelyMailConfig{Host: "host", FromAddress: "noreply@example.com"}},
		{"missing from address", PurelyMailConfig{Host: "host", Port: "465"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected a panic for missing required config")
				}
			}()
			_, _ = NewPurelyMail(tc.cfg)
		})
	}
}
