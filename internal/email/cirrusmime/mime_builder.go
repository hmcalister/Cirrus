package cirrusmime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
)

// rfc5322Date is the RFC 5322 date format, with a zero-padded day-of-month.
const rfc5322Date = "Mon, 02 Jan 2006 15:04:05 -0700"

type mimeBuilder interface {
	// buildEmail the full email (as bytes) from the common headers and message information.
	buildEmail(commonHeaders []string, msg email.Message) ([]byte, error)
}

// Render the message for the SMTP DATA command.
// Without attachments the message is plain text/plain.
// With attachments a multipart/mixed message is built whose first part is the text body.
func BuildMIMEMessage(fromAddress mail.Address, recipients email.Recipients, msg email.Message) ([]byte, error) {
	headers, err := buildCommonHeaders(fromAddress, recipients, msg)
	if err != nil {
		return nil, err
	}

	var builder mimeBuilder
	if len(msg.Attachments) == 0 {
		builder = plaintextMimeBuilder{}
	} else {
		builder = multipartMimeBuilder{}
	}
	return builder.buildEmail(headers, msg)
}

func buildCommonHeaders(fromAddress mail.Address, recipients email.Recipients, msg email.Message) ([]string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("email: generating message id: %w", err)
	}
	_, domain, _ := strings.Cut(fromAddress.Address, "@")

	headers := []string{
		"From: " + fromAddress.String(),
	}
	// Some servers flag mail without a To header as spam.
	// BCC recipients are never written to a header; they are passed as RCPT TO only.
	if to := formatAddressList(recipients.ToRecipients); to != "" {
		headers = append(headers, "To: "+to)
	}
	if cc := formatAddressList(recipients.CCRecipients); cc != "" {
		headers = append(headers, "Cc: "+cc)
	}
	headers = append(headers,
		"Subject: "+mime.QEncoding.Encode("utf-8", msg.Subject),
		"Date: "+time.Now().Format(rfc5322Date),
		fmt.Sprintf("Message-ID: <%s@%s>", hex.EncodeToString(id), domain),
		"MIME-Version: 1.0",
	)

	return headers, nil
}

// Join addresses into a comma-separated header value.
func formatAddressList(addrs []string) string {
	formatted := make([]string, 0, len(addrs))
	for _, raw := range addrs {
		a, err := mail.ParseAddress(raw)
		if err != nil {
			formatted = append(formatted, raw)
			continue
		}
		if a.Name == "" {
			formatted = append(formatted, a.Address)
			continue
		}
		formatted = append(formatted, a.String())
	}
	return strings.Join(formatted, ", ")
}

func normalizeCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
