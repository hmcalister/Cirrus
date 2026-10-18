package api

import "testing"

// Records the templates it is asked to render.
type mockPageRenderer struct {
	rendered []string
	body     string
	err      error
}

func (f *mockPageRenderer) RenderPage(templateName string, _ any) (string, error) {
	f.rendered = append(f.rendered, templateName)
	return f.body, f.err
}

func TestNewServerRequiresPages(t *testing.T) {
	t.Skip("scaffolding: enable once page handlers exist")

	linkBuilder, err := NewLinkBuilder("https://cloud.example.com")
	if err != nil {
		t.Fatal(err)
	}
	cfg := ServerConfig{
		TokenLinks:    linkBuilder,
		PagesRenderer: nil,
	}
	if _, err := NewServer(cfg); err == nil {
		t.Error("expected error for nil Pages, got nil")
	}
}

// TODO: remove when api has actual methods to test
func TestMockPageRenderer(t *testing.T) {
	renderer := &mockPageRenderer{body: "<html>ok</html>"}

	body, err := renderer.RenderPage("some_page.tmpl", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("RenderPage() err = %v, want nil", err)
	}
	if body != "<html>ok</html>" {
		t.Errorf("body = %q, want configured body", body)
	}
	if len(renderer.rendered) != 1 || renderer.rendered[0] != "some_page.tmpl" {
		t.Errorf("rendered = %v, want [some_page.tmpl]", renderer.rendered)
	}
}
