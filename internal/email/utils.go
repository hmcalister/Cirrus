package email

import (
	"errors"
	"mime"
	"net/mail"
	"path/filepath"
	"strings"
)

// Parse a single address and return the bare address.
func ParseAddress(s string) (string, error) {
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

// Returns att.ContentType, inferring it from the filename
// (or falling back to application/octet-stream) when it is empty.
func AttachmentContentType(att Attachment) string {
	if strings.TrimSpace(att.ContentType) != "" {
		return att.ContentType
	}
	if ct := mime.TypeByExtension(filepath.Ext(att.Filename)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// Returns the extension of name, including the leading dot.
func FilepathExtension(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i:]
	}
	return ""
}
