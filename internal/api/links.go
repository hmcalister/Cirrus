package api

import (
	"errors"
	"net/url"
	"strings"

	"github.com/hmcalister/cirrus/internal/authentication"
)

var _ authentication.TokenLinkBuilder = LinkBuilder{}

type LinkBuilder struct {
	BaseURL string
}

func NewLinkBuilder(baseURL string) (LinkBuilder, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return LinkBuilder{}, errors.New("BaseURL must not be empty")
	}
	if _, err := url.Parse(baseURL); err != nil {
		return LinkBuilder{}, err
	}
	return LinkBuilder{BaseURL: baseURL}, nil
}

func (b LinkBuilder) VerificationLink(rawToken string) string {
	return b.link("/verify", rawToken)
}

func (b LinkBuilder) PasswordResetLink(rawToken string) string {
	return b.link("/reset", rawToken)
}

func (b LinkBuilder) link(path, rawToken string) string {
	return b.BaseURL + path + "?token=" + url.QueryEscape(rawToken)
}
