package main

import (
	"fmt"
	"log"

	"github.com/wneessen/go-mail"
)

// sendEmail sends an alert email using the configuration
func sendEmail(subject, body string, cfg *Config) error {
	log.Printf("Attempting to send email alert: %s", subject)

	msg := mail.NewMsg()
	if err := msg.From(cfg.Email.From); err != nil {
		return fmt.Errorf("failed to set FROM address '%s': %w", cfg.Email.From, err)
	}
	if err := msg.To(cfg.Email.To); err != nil {
		return fmt.Errorf("failed to set TO address '%s': %w", cfg.Email.To, err)
	}

	msg.Subject(fmt.Sprintf("[ServMon Alert] %s", subject))
	msg.SetBodyString(mail.TypeTextPlain, body)

	// Create SMTP client with configuration
	client, err := mail.NewClient(
		cfg.Email.SMTPServer,
		mail.WithPort(cfg.Email.SMTPPort),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithUsername(cfg.Email.Username),
		mail.WithPassword(cfg.Email.Password),
	)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client for %s:%d: %w", cfg.Email.SMTPServer, cfg.Email.SMTPPort, err)
	}

	// Send the email
	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send email to %s via %s:%d: %w", cfg.Email.To, cfg.Email.SMTPServer, cfg.Email.SMTPPort, err)
	}

	log.Printf("✓ Email alert sent successfully to %s: %s", cfg.Email.To, subject)
	return nil
}
