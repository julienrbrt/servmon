package alert

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/wneessen/go-mail"
)

// EmailConfig holds email configuration
type EmailConfig struct {
	SMTPServer string
	SMTPPort   int
	From       string
	To         string
	Username   string
	Password   string
}

// EmailAlerter sends alerts via email
type EmailAlerter struct {
	config EmailConfig
}

// NewEmailAlerter creates a new email alerter
func NewEmailAlerter(config EmailConfig) *EmailAlerter {
	return &EmailAlerter{
		config: config,
	}
}

// Send sends an alert via email
func (e *EmailAlerter) Send(ctx context.Context, alert *Alert) error {
	log.Printf("Sending %s alert via email: %s", alert.Level, alert.Title)

	msg := mail.NewMsg()
	if err := msg.From(e.config.From); err != nil {
		return fmt.Errorf("failed to set FROM address '%s': %w", e.config.From, err)
	}
	if err := msg.To(e.config.To); err != nil {
		return fmt.Errorf("failed to set TO address '%s': %w", e.config.To, err)
	}

	subject := e.formatSubject(alert)
	msg.Subject(subject)

	body := e.formatBody(alert)
	msg.SetBodyString(mail.TypeTextPlain, body)

	// Create SMTP client with configuration
	client, err := mail.NewClient(
		e.config.SMTPServer,
		mail.WithPort(e.config.SMTPPort),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithUsername(e.config.Username),
		mail.WithPassword(e.config.Password),
	)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client for %s:%d: %w", e.config.SMTPServer, e.config.SMTPPort, err)
	}

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send email to %s via %s:%d: %w", e.config.To, e.config.SMTPServer, e.config.SMTPPort, err)
	}

	log.Printf("✓ Email alert sent successfully to %s: %s", e.config.To, alert.Title)
	return nil
}

// Name returns the name of the alerter
func (e *EmailAlerter) Name() string {
	return "Email"
}

// formatSubject formats the email subject with emoji and level
func (e *EmailAlerter) formatSubject(alert *Alert) string {
	var emoji string
	switch alert.Level {
	case LevelInfo:
		emoji = "ℹ️"
	case LevelWarning:
		emoji = "⚠️"
	case LevelCritical:
		emoji = "🚨"
	default:
		emoji = "📢"
	}

	return fmt.Sprintf("%s [ServMon %s] %s", emoji, alert.Level, alert.Title)
}

// formatBody formats the email body with detailed information
func (e *EmailAlerter) formatBody(alert *Alert) string {
	var sb strings.Builder

	// Header
	sb.WriteString("ServMon Alert\n")

	// Alert details
	sb.WriteString(fmt.Sprintf("Level: %s\n", alert.Level))
	sb.WriteString(fmt.Sprintf("Hostname: %s\n", alert.Hostname))
	sb.WriteString(fmt.Sprintf("Time: %s\n", alert.Timestamp.Format(time.RFC1123)))
	sb.WriteString("\n")

	// Title
	sb.WriteString(fmt.Sprintf("Title:\n%s\n\n", alert.Title))

	// Message
	sb.WriteString(fmt.Sprintf("Details:\n%s\n", alert.Message))

	// Metadata if present
	if len(alert.Metadata) > 0 {
		sb.WriteString("\nAdditional Information:\n")
		for key, value := range alert.Metadata {
			sb.WriteString(fmt.Sprintf("%s: %v\n", key, value))
		}
	}

	return sb.String()
}
