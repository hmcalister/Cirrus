package sender

import (
	"context"

	"github.com/hmcalister/Cirrus/internal/email"
)

// Sender sends a message to a list of recipients.
// Implementations must be safe for concurrent use.
type Sender interface {
	// Deliver msg to every recipient (as blind carbon copies: recipients do not see each other).
	Send(ctx context.Context, msg email.Message, recipients []string) error
}
