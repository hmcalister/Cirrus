// Handle emails in a pure fashion.
//
// Callers hand a Message and a list of recipients, and get back either nil or an error that can be inspected with errors.Is.
package email

// Message contains the content of an email.
type Message struct {
	// Subject line of the email.
	Subject string
	// Body of the email as a raw string. This may be empty.
	// New lines are converted to CRLF automatically.
	Body string
	// Attachments is an optional list of files to attach. A nil or empty slice
	// sends a plain text message.
	Attachments []Attachment
}

// Attachment is a single file attached to a Message.
type Attachment struct {
	// Filename is the name shown to the recipient (e.g. "report.pdf").
	Filename string
	// ContentType is the MIME type of the data (e.g. "application/pdf"). When
	// empty, buildMIME infers it from Filename, falling back to
	// application/octet-stream.
	ContentType string
	// Data is the raw (unencoded) content of the file.
	Data []byte
}

type ServerAuth struct {
	Host     string
	Port     string
	Username string
	Password string
}
