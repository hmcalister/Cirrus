package email

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/resend/resend-go/v3"
)

const (
	resend_cloudEmailHTMLSubject  string = "Your Daily Clouds"
	resend_cloudEmailHTMLTemplate string = "<html><body><h1>Your Daily Clouds</h1><p>Please find today's cloud image attached.</p></body></html>"
	// How many recipients to use in a BCC
	resend_emailBCCChunkSize    int           = 50
	resend_ratelimitResetPeriod time.Duration = 2 * time.Second
)

type ResendEmailManager struct {
	client      *resend.Client
	senderEmail string
	senderName  string
}

func NewResendEmailManager(apiKey, senderEmail, senderName, cloudSubject string) *ResendEmailManager {
	return &ResendEmailManager{
		client:      resend.NewClient(apiKey),
		senderEmail: senderEmail,
		senderName:  senderName,
	}
}

// This method attempts to send an email request for as long as the return error is rate limited.
// Any other returned error means the email request has failed. The error is returned.
func (r *ResendEmailManager) attemptDelivery(emailRequest *resend.SendEmailRequest) error {
	for {
		_, err := r.client.Emails.Send(emailRequest)
		if err == resend.ErrRateLimit {
			// If rate limited, try this recipient again after a short duration
			time.Sleep(resend_ratelimitResetPeriod)
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to send cloud email to %w", err)
		}
		return nil
	}
}

// Send the email to each recipient individually. This may incur significant charges if the number of recipients is large.
// Consider using sendChunkedBCCEmail to send emails to many recipients.
// This function overwrites the `To` field of the given emailRequest.
func (r *ResendEmailManager) sendEmail(recipientAddresses []string, emailRequest *resend.SendEmailRequest) error {
	if len(recipientAddresses) == 0 {
		return nil
	}

	allerrors := make([]error, 0)
	for _, recipient := range recipientAddresses {
		emailRequest.To = []string{recipient}
		err := r.attemptDelivery(emailRequest)
		if err != nil {
			allerrors = append(allerrors, err)
		}
	}

	if len(allerrors) == 0 {
		return nil
	}
	return errors.Join(allerrors...)
}

// Send an email to the recipients via the BCC mechanism to isolate users.
//
// This method will overwrite the `To` and `BCC` fields of the email request given.
func (r *ResendEmailManager) sendChunkedBCCEmail(recipientAddresses []string, emailRequest *resend.SendEmailRequest) error {
	if len(recipientAddresses) == 0 {
		return nil
	}

	allerrors := make([]error, 0)
	recipientChunks := slices.Chunk(recipientAddresses, resend_emailBCCChunkSize)
	for chunk := range recipientChunks {
		emailRequest.To = []string{r.senderEmail}
		emailRequest.Bcc = chunk
		err := r.attemptDelivery(emailRequest)
		if err != nil {
			allerrors = append(allerrors, err)
		}
	}

	if len(allerrors) == 0 {
		return nil
	}
	return errors.Join(allerrors...)
}

// SendCloudEmail sends the daily cloud image to each recipient as a BCC.
func (r *ResendEmailManager) SendCloudEmail(recipientAddresses []string, cloudImage []byte) error {
	from := fmt.Sprintf("%s <%s>", r.senderName, r.senderEmail)
	// The `To` and `BCC` fields are set by `sendChunkedBCCEmail`.
	emailRequest := &resend.SendEmailRequest{
		From:    from,
		Subject: resend_cloudEmailHTMLSubject,
		Html:    resend_cloudEmailHTMLTemplate,
		Attachments: []*resend.Attachment{
			{
				Content:  cloudImage,
				Filename: "cloud.jpg",
			},
		},
	}
	err := r.sendChunkedBCCEmail(recipientAddresses, emailRequest)
	return err
}

// SendGeneralEmail sends a general text message to each recipient individually.
func (r *ResendEmailManager) SendGeneralEmail(recipientAddresses []string, message string) error {
	from := fmt.Sprintf("%s <%s>", r.senderName, r.senderEmail)
	// The `To` field set by `sendEmail`.
	emailRequest := &resend.SendEmailRequest{
		From:    from,
		Subject: "Message from Literal Cloud Service",
		Text:    message,
	}
	err := r.sendEmail(recipientAddresses, emailRequest)
	return err
}
