package sender

import (
	"context"

	"github.com/hmcalister/Cirrus/internal/email"
)

type SendParams struct {
	email.Message
	email.Recipients
}

// Sender sends a message to a list of recipients.
// Implementations must be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, params SendParams) error
}
