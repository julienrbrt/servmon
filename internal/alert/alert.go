package alert

import (
	"context"
	"os"
	"time"
)

// Alerter is an interface for sending alerts
type Alerter interface {
	// Send sends an alert
	Send(ctx context.Context, alert *Alert) error
	// Name returns the name of the alerter
	Name() string
}

// AlertLevel represents the severity of an alert
type AlertLevel string

const (
	LevelInfo     AlertLevel = "INFO"
	LevelWarning  AlertLevel = "WARNING"
	LevelCritical AlertLevel = "CRITICAL"
)

// Alert represents a monitoring alert
type Alert struct {
	Level     AlertLevel
	Title     string
	Message   string
	Timestamp time.Time
	Hostname  string
	Metadata  map[string]any
}

// NewAlert creates a new alert with hostname and timestamp
func NewAlert(level AlertLevel, title, message string) *Alert {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return &Alert{
		Level:     level,
		Title:     title,
		Message:   message,
		Timestamp: time.Now(),
		Hostname:  hostname,
		Metadata:  make(map[string]any),
	}
}

// WithMetadata adds metadata to the alert
func (a *Alert) WithMetadata(key string, value any) *Alert {
	a.Metadata[key] = value
	return a
}
