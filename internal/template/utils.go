package template

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"strings"
	texttemplate "text/template"
)

func requireTemplates(fsys fs.FS, dir string, required []string) error {
	var errs []error
	for _, name := range required {
		path := dir + "/" + strings.TrimPrefix(name, "/")
		if _, err := fs.Stat(fsys, path); err != nil {
			errs = append(errs, fmt.Errorf("template: required template %q: %w", path, err))
		}
	}
	return errors.Join(errs...)
}

func parseTextTemplates(fsys fs.FS, dir string) (*texttemplate.Template, error) {
	pattern := dir + "/" + templatePattern
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("template: listing %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return texttemplate.New("email"), nil
	}

	root := texttemplate.New("email")
	root, err = root.ParseFS(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("template: parsing %q: %w", pattern, err)
	}
	return root, nil
}

func parseHTMLTemplates(fsys fs.FS, dir string) (*template.Template, error) {
	pattern := dir + "/" + templatePattern
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("template: listing %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return template.New("web"), nil
	}

	root := template.New("web")
	root, err = root.ParseFS(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("template: parsing %q: %w", pattern, err)
	}
	return root, nil
}
