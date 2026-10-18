package template

// Tests for the template loader.
//
// Approach:
//
//  1. Unit tests use fstest.MapFS so the templates live inside the test file and
//     no disk layout is assumed. Pass a MapFS to NewLoader exactly as production
//     passes the embedded FS.
//  2. Golden tests execute a template into a bytes.Buffer and compare against a
//     file in testdata/. Regenerate them with `go test ./internal/template -update`.
//     Suggested layout: testdata/<case>/want.txt (email) or want.html (page).
//  3. Escaping is an html/template concern; assert it here so a future move to
//     plain text/template for pages fails loudly.
//  4. Plain-text email bodies are asserted here. CRLF normalisation is NOT the
//     loader's job; it is verified at the cirrusmime boundary.
//  5. A last test parses the real embedded FS so fixtures cannot drift from what
//     ships. Because templates are embedded, this is exact: NewLoader(FS, ...),
//     with no path resolution. Keep the required lists in sync with
//     cmd/server/main.go.

import "testing"

// TestNewParsesAllTemplates parses one valid template per kind.
// Uses fstest.MapFS with an email template and a web template, then calls
// NewLoader and renders both to confirm they were parsed and are resolvable.
func TestNewParsesAllTemplates(t *testing.T) {
	t.Skip("scaffolding: enable once templates are authored")
}

// TestNewMissingRequired covers the only startup validation: every name in a
// non-empty required slice must exist. Build a MapFS that omits the required
// name and assert the returned error names the missing path. Table cases:
// missing email template, missing web template, both missing (assert the
// errors.Join result mentions each). Also verify that an empty required slice
// does not trigger this check.
func TestNewMissingRequired(t *testing.T) {
	t.Skip("scaffolding: enable once templates are authored")
}

// TestNewEmptyDirectoryPasses pins the agreed rule: an empty directory is valid
// when nothing is required from it. Pass an empty fstest.MapFS with empty
// required slices and expect NewLoader to succeed; then expect RenderEmail to
// report that the name was not found.
func TestNewEmptyDirectoryPasses(t *testing.T) {
	t.Skip("scaffolding: enable once templates are authored")
}

// TestRenderEmailPlaintext executes an email template containing newlines and
// asserts the exact body. Include a note that CRLF is applied later by
// cirrusmime.BuildMIMEMessage; this test is about fidelity before that step.
func TestRenderEmailPlaintext(t *testing.T) {
	t.Skip("scaffolding: enable once templates are authored")
}

// TestRenderPageEscapes injects a value such as "<script>alert(1)</script>" and
// asserts the rendered HTML contains the escaped form. Proves pages use
// html/template, not text/template.
func TestRenderPageEscapes(t *testing.T) {
	t.Skip("scaffolding: enable once templates are authored")
}

// TestGolden is the golden-file harness. Table cases name a fixture directory,
// a template name, and data; each case renders into a bytes.Buffer and compares
// to testdata/<case>/want.{txt,html}. Testdata files hold operator-visible
// content, so they are the regression net for template design changes.
func TestGolden(t *testing.T) {
	t.Skip("scaffolding: enable once templates are authored")
}

// TestRealTemplatesParse parses the embedded FS so that testdata fixtures cannot
// drift from what is compiled into the binary: NewLoader(FS, requiredEmail,
// requiredWeb). Because the FS is embedded, no path resolution is needed.
// Remember that the required lists passed here must stay in sync with
// cmd/server/main.go.
func TestRealTemplatesParse(t *testing.T) {
	t.Skip("scaffolding: enable once templates are authored")
}
