package api

import (
	"io"
	"testing"
)

// Records the templates it is asked to render.
type mockPageRenderer struct {
	rendered []string
	err      error
}

func (f *mockPageRenderer) RenderPage(_ io.Writer, templateName string, _ any) error {
	f.rendered = append(f.rendered, templateName)
	return f.err
}

func TestNewServerRequiresPages(t *testing.T) {
	t.Skip("scaffolding: enable once page handlers exist")

	linkBuilder, err := NewLinkBuilder("https://cloud.example.com")
	if err != nil {
		t.Fatal(err)
	}
	cfg := ServerConfig{
		TokenLinks:     linkBuilder,
		PagesTemplates: nil,
	}
	if _, err := NewServer(cfg); err == nil {
		t.Error("expected error for nil Pages, got nil")
	}
}
