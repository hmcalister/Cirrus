package sender

import (
	"testing"
	"time"

	"github.com/hmcalister/Cirrus/internal/email/mailer"
)

func TestNewSMTPRejectsBadFrom(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic for missing required config")
		}
	}()
	mailerServerAuth, _ := mailer.NewServerAuth(
		"smtp.example.com",
		"465",
		"user",
		"secret",
	)
	_, err := NewSMTPSender(
		SMTPConfig{
			ServerAuth:  mailerServerAuth,
			FromAddress: "not-an-address",
			Timeout:     time.Second,
		})
	if err != nil {
		t.Fatalf("got error %v, wanted nil", err)
	}
}
