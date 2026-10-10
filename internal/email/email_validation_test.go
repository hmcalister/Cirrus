package email

import (
	"testing"
)

// TestAttachmentContentTypeInference checks the fallback chain for the MIME
// type of an attachment.
func TestAttachmentContentTypeInference(t *testing.T) {
	tests := []struct {
		name string
		att  Attachment
		want string
	}{
		{"explicit", Attachment{ContentType: "application/pdf"}, "application/pdf"},
		{"from extension", Attachment{Filename: "report.pdf"}, "application/pdf"},
		{"unknown extension", Attachment{Filename: "file.unknownext"}, "application/octet-stream"},
		{"no extension", Attachment{Filename: "Makefile"}, "application/octet-stream"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := AttachmentContentType(tc.att); got != tc.want {
				t.Errorf("attachmentContentType = %q, want %q", got, tc.want)
			}
		})
	}
}
