package main

import (
	"fmt"
	"log"

	"github.com/wneessen/go-mail"
)

// sendEmail sends an alert email using the configuration
func sendEmail(subject, body string, cfg *Config) error {
	msg := mail.NewMsg()
	if err := msg.From(cfg.Email.From); err != nil {
		return fmt.Errorf("failed to set FROM address: %w", err)
	}
	if err := msg.To(cfg.Email.To); err != nil {
		return fmt.Errorf("failed to set TO address: %w", err)
	}

	msg.Subject(fmt.Sprintf("[ServMon Alert] %s", subject))
	msg.SetBodyString(mail.TypeTextPlain, body)

	// Create SMTP client with configuration
	client, err := mail.NewClient(
		cfg.Email.SMTPServer,
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithUsername(cfg.Email.Username),
		mail.WithPassword(cfg.Email.Password),
	)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	// Send the email
	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email alert sent successfully: %s", subject)
	return nil
}
