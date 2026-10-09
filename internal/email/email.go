// Handle emails in a pure fashion.
//
// Callers hand a Message and a list of recipients, and get back either nil or an error that can be inspected with errors.Is.
package email

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"time"
)

// Sender sends a message to a list of recipients.
//
// Implementations must be safe for concurrent use.
type Sender interface {
	// Deliver msg to every recipient (as blind carbon copies: recipients do not see each other).
	Send(ctx context.Context, msg Message, recipients []string) error
}

// Message contains the content of an email.
type Message struct {
	Subject string
	Body    string
}

// Check a message. Returns an error if any part of the message is invalid.
func validateMessage(msg Message) error {
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

	return nil
}

// Check a recipients list. Returns the bare addresses (display names stripped), or an error if any are invalid.
func validateRecipients(recipients []string) ([]string, error) {
	if len(recipients) == 0 {
		return nil, ErrNoRecipients
	}

	addrs := make([]string, 0, len(recipients))
	for _, r := range recipients {
		addr, err := parseAddress(r)
		if err != nil {
			return nil, fmt.Errorf("%w: %q: %v", ErrInvalidRecipient, r, err)
		}
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

// Render the message for the SMTP DATA command.
// Recipients are not written into the headers (they are passed as RCPT TO only), so they are blind-copied.
func buildMIME(fromName, fromAddress string, msg Message) ([]byte, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("email: generating message id: %w", err)
	}
	_, domain, _ := strings.Cut(fromAddress, "@")
	fromHeaderString := (&mail.Address{Name: fromName, Address: fromAddress}).String()

	headers := []string{
		"From: " + fromHeaderString,
		// Some servers flag mail without a To header as spam.
		"To: " + fromHeaderString,
		"Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject),
		"Date: " + time.Now().Format(rfc5322Date),
		fmt.Sprintf("Message-ID: <%s@%s>", hex.EncodeToString(id[:]), domain),
		"MIME-Version: 1.0",
		`Content-Type: text/plain; charset="utf-8"`,
		"Content-Transfer-Encoding: 8bit",
	}

	body := normalizeCRLF(msg.Body)
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body), nil
}
