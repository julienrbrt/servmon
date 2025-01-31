package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is a struct that holds the configuration for the monitoring service.
type Config struct {
	AlertThresholds Thresholds `yaml:"alert_thresholds"`
	Email           Email      `yaml:"email"`
}

type Thresholds struct {
	CPU    ThresholdConfig `yaml:"cpu"`
	Memory ThresholdConfig `yaml:"memory"`
	Disk   ThresholdConfig `yaml:"disk"`
	HTTP   HTTP            `yaml:"http"`
}

type ThresholdConfig struct {
	Threshold float64       `yaml:"threshold"`
	Duration  time.Duration `yaml:"duration,omitempty"`
	Cooldown  time.Duration `yaml:"cooldown"`
}

type HTTP struct {
	URL              string        `yaml:"url"`
	Timeout          time.Duration `yaml:"timeout"`
	SampleRate       int           `yaml:"sample_rate"`
	FailureThreshold float64       `yaml:"failure_threshold"`
	CheckInterval    time.Duration `yaml:"check_interval"`
	Cooldown         time.Duration `yaml:"cooldown"`
}

type Email struct {
	SMTPServer string `yaml:"smtp_server"`
	From       string `yaml:"from"`
	To         string `yaml:"to"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}

func (c *Config) Save(path string) error {
	out, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("error generating sample config: %w", err)
	}

	return nil
}

// defaultConfig returns a default configuration for the monitoring service.
func defaultConfig() *Config {
	return &Config{
		AlertThresholds: Thresholds{
			CPU: ThresholdConfig{
				Threshold: 90,
				Duration:  5 * time.Minute,
				Cooldown:  30 * time.Minute,
			},
			Memory: ThresholdConfig{
				Threshold: 80,
				Cooldown:  30 * time.Minute,
			},
			Disk: ThresholdConfig{
				Threshold: 90,
				Cooldown:  4 * time.Hour,
			},
			HTTP: HTTP{
				URL:              "http://localhost:8080/health",
				Timeout:          5 * time.Second,
				SampleRate:       10,
				FailureThreshold: 20,
				CheckInterval:    1 * time.Minute,
				Cooldown:         15 * time.Minute,
			},
		},
		Email: Email{
			SMTPServer: "smtp.example.com",
			From:       "alerts@example.com",
			To:         "admin@example.com",
			Username:   "alertuser",
			Password:   "alertpassword",
		},
	}
}

// loadConfig loads a configuration from a file.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}
