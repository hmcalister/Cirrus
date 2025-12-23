package email

type EmailManager interface {
	// Send the daily cloud email. It is up to the email manager implementation how exactly
	// the email is sent (BCC, backoffs, etc) but each recipient should receive exactly one email,
	// and should not have other recipient's addresses revealed to them.
	//
	// WARNING: This method may incur a charge if the implementation uses an email provider API!
	SendCloudEmail(recipientAddresses []string, cloudImage []byte) (err error)

	// Expose a very generic email sending method. This method should only be used in rare circumstances,
	// such as updates to admins. If you are using this method, consider very carefully if another method
	// could be used.
	//
	// WARNING: This method may incur a charge if the implementation uses an email provider API!
	SendGeneralEmail(recipientAddresses []string, message string) (err error)
}
