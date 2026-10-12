package email

// Recipients of all forms
type Recipients struct {
	ToRecipients  []string
	CCRecipients  []string
	BCCRecipients []string
}

// Message contains the content of an email.
type Message struct {
	// Subject line of the email.
	Subject string
	// Body of the email as a raw string. This may be empty.
	// New lines are converted to CRLF.
	Body string
	// Attachments is an optional list of files to attach.
	// A nil or empty slice sends a plain text message.
	Attachments []Attachment
}

// Attachment is a single file attached to a Message.
type Attachment struct {
	// Filename is the name shown to the recipient (e.g. "report.pdf").
	Filename string
	// ContentType is the MIME type of the data (e.g. "application/pdf").
	// When empty, buildMIME infers it from Filename.
	// Falls back to application/octet-stream.
	ContentType string
	// Data is the raw (unencoded) content of the file.
	Data []byte
}
