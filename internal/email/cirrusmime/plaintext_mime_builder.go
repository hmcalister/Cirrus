package cirrusmime

import (
	"strings"

	"github.com/hmcalister/Cirrus/internal/email"
)

var _ mimeBuilder = plaintextMimeBuilder{}

type plaintextMimeBuilder struct{}

func (builder plaintextMimeBuilder) buildEmail(commonHeaders []string, msg email.Message) ([]byte, error) {
	body := normalizeCRLF(msg.Body)
	return []byte(strings.Join(commonHeaders, "\r\n") + "\r\n\r\n" + body), nil
}
