package cirrusmime

import (
	"strings"

	"github.com/hmcalister/Cirrus/internal/email"
)

var _ emailBuilder = plaintextBuilder{}

type plaintextBuilder struct{}

func (builder plaintextBuilder) buildEmail(commonHeaders []string, msg email.Message) ([]byte, error) {
	body := normalizeCRLF(msg.Body)
	return []byte(strings.Join(commonHeaders, "\r\n") + "\r\n\r\n" + body), nil
}
