package email

import (
	"errors"
	"fmt"
	"strings"
)

// Check a message. Returns an error if any part of the message is invalid.
func ValidateMessage(msg Message) error {
	if strings.TrimSpace(msg.Subject) == "" {
		return fmt.Errorf("%w: subject is empty", ErrInvalidMessage)
	}
	// Line breaks in a header value allow header injection.
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return fmt.Errorf("%w: subject contains a line break", ErrInvalidMessage)
	}
	if strings.TrimSpace(msg.Body) == "" {
		return fmt.Errorf("%w: body is empty", ErrInvalidMessage)
	}
	for i, att := range msg.Attachments {
		if err := ValidateAttachment(att); err != nil {
			return fmt.Errorf("%w: attachment %d: %w", ErrInvalidMessage, i, err)
		}
	}

	return nil
}

// Check a single attachment. Returns an error if it is unusable.
func ValidateAttachment(att Attachment) error {
	if strings.TrimSpace(att.Filename) == "" {
		return errors.New("filename is empty")
	}
	// Line breaks in a header value allow header injection.
	if strings.ContainsAny(att.Filename, "\r\n") {
		return errors.New("filename contains a line break")
	}
	if strings.ContainsAny(att.ContentType, "\r\n") {
		return errors.New("content type contains a line break")
	}
	if len(att.Data) == 0 {
		return errors.New("data is empty")
	}

	return nil
}

// Check a recipients list. Returns the bare addresses (display names stripped), or an error if any are invalid.
func ValidateRecipients(recipients []string) ([]string, error) {
	if len(recipients) == 0 {
		return nil, ErrNoRecipients
	}

	addrs := make([]string, 0, len(recipients))
	for _, r := range recipients {
		addr, err := ParseAddress(r)
		if err != nil {
			return nil, fmt.Errorf("%w: %q: %w", ErrInvalidRecipient, r, err)
		}
		addrs = append(addrs, addr)
	}

	return addrs, nil
}
