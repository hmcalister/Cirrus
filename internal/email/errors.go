package email

import (
	"errors"
	"fmt"
	"net/textproto"
)

var (
	// Message is unusable (e.g. no subject or body).
	ErrInvalidMessage = errors.New("email: invalid message")
	// An attachment is larger than the allowed limit.
	ErrAttachmentTooLarge = errors.New("email: attachment too large")
	// Recipient is empty or not a valid address.
	ErrInvalidRecipient = errors.New("email: invalid recipient")
	// The recipient list was empty.
	ErrNoRecipients = errors.New("email: no recipients")
	// The connection to the mail server could not be established or was lost.
	ErrConnection = errors.New("email: connection failed")
	// The mail server rejected auth credentials.
	ErrAuthentication = errors.New("email: authentication failed")
	// The mail server refused the message.
	ErrDelivery = errors.New("email: delivery failed")
)

// RecipientError reports that a single recipient was rejected by the mail
// server during RCPT TO. It wraps ErrDelivery (or ErrConnection if the
// connection was lost while attempting the recipient).
type RecipientError struct {
	Recipient string
	Err       error
}

func (e *RecipientError) Error() string {
	return fmt.Sprintf("email: recipient %q: %v", e.Recipient, e.Err)
}

func (e *RecipientError) Unwrap() error {
	return e.Err
}

// wrapSMTPError classifies an SMTP protocol error: a server response is a
// delivery failure, while any other error (e.g. a broken connection) is a
// connection failure.
func WrapSMTPError(err error) error {
	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) {
		return fmt.Errorf("%w: %w", ErrDelivery, err)
	}
	return fmt.Errorf("%w: %w", ErrConnection, err)
}
