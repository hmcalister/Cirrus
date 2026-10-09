package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hmcalister/Cirrus/internal/email"
)

const (
	smtpHost  = "smtp.example.com"
	smtpPort  = "465"
	username  = "REPLACE_ME"
	password  = "REPLACE_ME"
	fromName  = "Literal Cloud Test"
	fromAddr  = "REPLACE_ME"
	recipient = "REPLACE_ME"
)

func main() {
	sender, err := email.NewSMTPSender(email.SMTPConfig{
		Host:        smtpHost,
		Port:        smtpPort,
		Username:    username,
		Password:    password,
		FromAddress: fromAddr,
		FromName:    fromName,
		Timeout:     30 * time.Second,
	})
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msg := email.Message{
		Subject: "SMTP smoke test",
		Body:    "hello from the debug script\n",
	}

	if err := sender.Send(ctx, msg, []string{recipient}); err != nil {
		switch {
		case errors.Is(err, email.ErrConnection):
			log.Fatalf("connection failed: %v", err)
		case errors.Is(err, email.ErrAuthentication):
			log.Fatalf("authentication failed: %v", err)
		case errors.Is(err, email.ErrDelivery):
			log.Fatalf("delivery failed: %v", err)
		case errors.Is(err, email.ErrInvalidMessage):
			log.Fatalf("invalid message: %v", err)
		case errors.Is(err, email.ErrInvalidRecipient):
			log.Fatalf("invalid recipient: %v", err)
		case errors.Is(err, email.ErrNoRecipients):
			log.Fatalf("no recipients: %v", err)
		default:
			log.Fatalf("send failed: %v", err)
		}
	}

	fmt.Printf("sent OK to %s\n", recipient)
}