package email

import (
	"errors"
	"net/mail"
	"strings"
)

// rfc5322Date is the RFC 5322 date format, with a zero-padded day-of-month.
const rfc5322Date = "Mon, 02 Jan 2006 15:04:05 -0700"

// normalizeCRLF converts bare LF (and CR) line endings to CRLF as required by
// SMTP, leaving existing CRLF sequences untouched.
func normalizeCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

// Parse a single address and return the bare address.
func parseAddress(s string) (string, error) {
	// Line breaks in a header value allow header injection.
	if strings.ContainsAny(s, "\r\n") {
		return "", errors.New("contains a line break")
	}
	a, err := mail.ParseAddress(s)
	if err != nil {
		return "", err
	}

	return a.Address, nil
}
