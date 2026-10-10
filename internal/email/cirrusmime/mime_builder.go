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

type emailBuilder interface {
	// buildEmail the full email (as bytes) from the common headers and message information.
	buildEmail(commonHeaders []string, msg email.Message) ([]byte, error)
}

// Render the message for the SMTP DATA command.
// Without attachments the message is plain text/plain; with attachments it is
// a multipart/mixed message whose first part is the text body.
func BuildMIMEMessage(fromAddress mail.Address, msg email.Message) ([]byte, error) {
	headers, err := buildCommonHeaders(fromAddress, msg)
	if err != nil {
		return nil, err
	}

	var builder emailBuilder
	if len(msg.Attachments) == 0 {
		builder = plaintextBuilder{}
	} else {
		builder = multipartBuilder{}
	}
	return builder.buildEmail(headers, msg)
}

func buildCommonHeaders(fromAddress mail.Address, msg email.Message) ([]string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("email: generating message id: %w", err)
	}
	_, domain, _ := strings.Cut(fromAddress.Address, "@")
	fromHeaderString := fromAddress.String()

	headers := []string{
		"From: " + fromHeaderString,
		// Some servers flag mail without a To header as spam.
		// Recipients are not written into the headers (they are passed as RCPT TO only), so they are blind-copied.
		"To: " + fromHeaderString,
		"Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject),
		"Date: " + time.Now().Format(rfc5322Date),
		fmt.Sprintf("Message-ID: <%s@%s>", hex.EncodeToString(id), domain),
		"MIME-Version: 1.0",
	}

	return headers, nil
}

// normalizeCRLF converts bare LF (and CR) line endings to CRLF as required by
// SMTP, leaving existing CRLF sequences untouched.
func normalizeCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
