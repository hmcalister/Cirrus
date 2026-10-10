package cirrusmime

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/hmcalister/Cirrus/internal/email"
)

var _ emailBuilder = multipartBuilder{}

type multipartBuilder struct{}

func (builder multipartBuilder) buildEmail(commonHeaders []string, msg email.Message) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", `text/plain; charset="utf-8"`)
	textHeader.Set("Content-Transfer-Encoding", "8bit")
	textPart, err := w.CreatePart(textHeader)
	if err != nil {
		return nil, fmt.Errorf("email: building body part: %w", err)
	}
	if _, err := textPart.Write([]byte(normalizeCRLF(msg.Body))); err != nil {
		return nil, fmt.Errorf("email: writing body part: %w", err)
	}

	for i, att := range msg.Attachments {
		if err := writeAttachmentPart(w, att); err != nil {
			return nil, fmt.Errorf("email: building attachment %d: %w", i, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("email: closing multipart writer: %w", err)
	}

	headers := append(commonHeaders, `Content-Type: multipart/mixed; boundary="`+w.Boundary()+`"`)

	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + buf.String()), nil
}

// writeAttachmentPart writes a single attachment as a base64-encoded part.
func writeAttachmentPart(w *multipart.Writer, att email.Attachment) error {
	h := textproto.MIMEHeader{}
	h.Set("Content-Type", email.AttachmentContentType(att))
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": att.Filename}))
	h.Set("Content-Transfer-Encoding", "base64")

	part, err := w.CreatePart(h)
	if err != nil {
		return err
	}

	// Base64 lines must be wrapped; Go's base64 does not wrap on its own.
	enc := base64.NewEncoder(base64.StdEncoding, &crlfLineWriter{w: part})
	if _, err := enc.Write(att.Data); err != nil {
		return err
	}
	return enc.Close()
}

// crlfLineWriter wraps base64 output at 76 characters and separates lines with
// CRLF, as required for a MIME body part.
type crlfLineWriter struct {
	w     io.Writer
	count int
}

func (c *crlfLineWriter) Write(p []byte) (int, error) {
	const lineWidth = 76
	written := 0
	for len(p) > 0 {
		n := lineWidth - c.count
		if n > len(p) {
			n = len(p)
		}
		if c.count == lineWidth {
			if _, err := c.w.Write([]byte("\r\n")); err != nil {
				return written, err
			}
			c.count = 0
			continue
		}
		if _, err := c.w.Write(p[:n]); err != nil {
			return written, err
		}
		written += n
		c.count += n
		p = p[n:]
	}
	return written, nil
}
