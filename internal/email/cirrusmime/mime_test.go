package cirrusmime

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"testing"

	"github.com/hmcalister/Cirrus/internal/email"
)

func TestAttachmentBase64Wrapping(t *testing.T) {
	data := bytes.Repeat([]byte{0xab}, 300) // 400 base64 chars, spans multiple lines
	msg := email.Message{
		Subject:     "s",
		Body:        "b",
		Attachments: []email.Attachment{{Filename: "big.bin", ContentType: "application/octet-stream", Data: data}},
	}

	rendered, err := BuildMIMEMessage(mail.Address{
		Name:    "Cloud",
		Address: "noreply@example.com",
	}, email.Recipients{ToRecipients: []string{"recipient@example.com"}}, msg)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(rendered))
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(parsed.Body, params["boundary"])
	if _, err := mr.NextPart(); err != nil { // skip the text part
		t.Fatal(err)
	}
	part, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}

	encoded, _ := io.ReadAll(part)
	decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("decoded payload does not match the input data")
	}

	// The raw encoding must use short CRLF-terminated lines.
	for _, line := range bytes.Split(encoded, []byte("\r\n")) {
		if len(line) > 76 {
			t.Errorf("base64 line length = %d, want <= 76", len(line))
		}
	}
}
