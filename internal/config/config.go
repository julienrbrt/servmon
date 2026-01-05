package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is a struct that holds the configuration for the monitoring service.
type Config struct {
	AlertThresholds Thresholds `yaml:"alert_thresholds"`
	Email           Email      `yaml:"email"`
}

type Thresholds struct {
	CPU        ThresholdConfig  `yaml:"cpu"`
	Memory     ThresholdConfig  `yaml:"memory"`
	Disks      []DiskConfig     `yaml:"disks"`
	HTTP       HTTP             `yaml:"http"`
	Journalctl JournalctlConfig `yaml:"journalctl"`
	Reboot     RebootConfig     `yaml:"reboot"`
}

type ThresholdConfig struct {
	Threshold     float64       `yaml:"threshold"`
	Duration      time.Duration `yaml:"duration,omitempty"`
	Cooldown      time.Duration `yaml:"cooldown"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

type DiskConfig struct {
	Path          string        `yaml:"path"`
	Threshold     float64       `yaml:"threshold"`
	Cooldown      time.Duration `yaml:"cooldown"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

type HTTP struct {
	URL              string        `yaml:"url"`
	Timeout          time.Duration `yaml:"timeout"`
	SampleRate       int           `yaml:"sample_rate"`
	FailureThreshold float64       `yaml:"failure_threshold"`
	CheckInterval    time.Duration `yaml:"check_interval"`
	Cooldown         time.Duration `yaml:"cooldown"`
}

type JournalctlConfig struct {
	Enabled        bool          `yaml:"enabled"`
	CheckInterval  time.Duration `yaml:"check_interval"`
	LookbackPeriod time.Duration `yaml:"lookback_period"`
	ErrorThreshold int           `yaml:"error_threshold"`
	Priorities     []string      `yaml:"priorities"` // err, crit, alert, emerg
	Cooldown       time.Duration `yaml:"cooldown"`
}

type RebootConfig struct {
	Enabled         bool          `yaml:"enabled"`
	UptimeThreshold time.Duration `yaml:"uptime_threshold"` // If uptime < threshold, send reboot notification
}

type Email struct {
	SMTPServer string `yaml:"smtp_server"`
	SMTPPort   int    `yaml:"smtp_port"`
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

// Default returns a default configuration for the monitoring service.
func Default() *Config {
	return &Config{
		AlertThresholds: Thresholds{
			CPU: ThresholdConfig{
				Threshold:     90,
				Duration:      5 * time.Minute,
				Cooldown:      30 * time.Minute,
				CheckInterval: 10 * time.Second,
			},
			Memory: ThresholdConfig{
				Threshold:     80,
				Cooldown:      30 * time.Minute,
				CheckInterval: 10 * time.Second,
			},
			Disks: []DiskConfig{
				{
					Path:          "/",
					Threshold:     90,
					Cooldown:      4 * time.Hour,
					CheckInterval: 1 * time.Minute,
				},
			},
			HTTP: HTTP{
				URL:              "http://localhost:8080/health",
				Timeout:          5 * time.Second,
				SampleRate:       10,
				FailureThreshold: 20,
				CheckInterval:    1 * time.Minute,
				Cooldown:         15 * time.Minute,
			},
			Journalctl: JournalctlConfig{
				Enabled:        true,
				CheckInterval:  5 * time.Minute,
				LookbackPeriod: 5 * time.Minute,
				ErrorThreshold: 10,
				Priorities:     []string{"err", "crit", "alert", "emerg"},
				Cooldown:       30 * time.Minute,
			},
			Reboot: RebootConfig{
				Enabled:         true,
				UptimeThreshold: 10 * time.Minute,
			},
		},
		Email: Email{
			SMTPServer: "smtp.example.com",
			SMTPPort:   587,
			From:       "alerts@example.com",
			To:         "admin@example.com",
			Username:   "alertuser",
			Password:   "alertpassword",
		},
	}
}

// Load loads a configuration from a file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate CPU thresholds
	if c.AlertThresholds.CPU.Threshold <= 0 || c.AlertThresholds.CPU.Threshold > 100 {
		return fmt.Errorf("CPU threshold must be between 0 and 100")
	}
	if c.AlertThresholds.CPU.Cooldown <= 0 {
		return fmt.Errorf("CPU cooldown must be positive")
	}
	if c.AlertThresholds.CPU.CheckInterval <= 0 {
		return fmt.Errorf("CPU check interval must be positive")
	}

	// Validate Memory thresholds
	if c.AlertThresholds.Memory.Threshold <= 0 || c.AlertThresholds.Memory.Threshold > 100 {
		return fmt.Errorf("memory threshold must be between 0 and 100")
	}
	if c.AlertThresholds.Memory.Cooldown <= 0 {
		return fmt.Errorf("memory cooldown must be positive")
	}
	if c.AlertThresholds.Memory.CheckInterval <= 0 {
		return fmt.Errorf("memory check interval must be positive")
	}

	// Validate Disk thresholds
	if len(c.AlertThresholds.Disks) == 0 {
		return fmt.Errorf("at least one disk must be configured")
	}
	for i, disk := range c.AlertThresholds.Disks {
		if disk.Path == "" {
			return fmt.Errorf("disk[%d] path cannot be empty", i)
		}
		if disk.Threshold <= 0 || disk.Threshold > 100 {
			return fmt.Errorf("disk[%d] threshold must be between 0 and 100", i)
		}
		if disk.Cooldown <= 0 {
			return fmt.Errorf("disk[%d] cooldown must be positive", i)
		}
		if disk.CheckInterval <= 0 {
			return fmt.Errorf("disk[%d] check interval must be positive", i)
		}
	}

	// Validate HTTP thresholds
	if c.AlertThresholds.HTTP.URL != "" {
		if !strings.HasPrefix(c.AlertThresholds.HTTP.URL, "http://") && !strings.HasPrefix(c.AlertThresholds.HTTP.URL, "https://") {
			return fmt.Errorf("HTTP URL must start with http:// or https://")
		}
		if c.AlertThresholds.HTTP.Timeout <= 0 {
			return fmt.Errorf("HTTP timeout must be positive")
		}
		if c.AlertThresholds.HTTP.SampleRate <= 0 {
			return fmt.Errorf("HTTP sample rate must be positive")
		}
		if c.AlertThresholds.HTTP.FailureThreshold < 0 || c.AlertThresholds.HTTP.FailureThreshold > 100 {
			return fmt.Errorf("HTTP failure threshold must be between 0 and 100")
		}
		if c.AlertThresholds.HTTP.CheckInterval <= 0 {
			return fmt.Errorf("HTTP check interval must be positive")
		}
		if c.AlertThresholds.HTTP.Cooldown <= 0 {
			return fmt.Errorf("HTTP cooldown must be positive")
		}
	}

	// Validate Journalctl configuration
	if c.AlertThresholds.Journalctl.Enabled {
		if c.AlertThresholds.Journalctl.CheckInterval <= 0 {
			return fmt.Errorf("journalctl check interval must be positive")
		}
		if c.AlertThresholds.Journalctl.LookbackPeriod <= 0 {
			return fmt.Errorf("journalctl lookback period must be positive")
		}
		if c.AlertThresholds.Journalctl.ErrorThreshold <= 0 {
			return fmt.Errorf("journalctl error threshold must be positive")
		}
		if len(c.AlertThresholds.Journalctl.Priorities) == 0 {
			return fmt.Errorf("journalctl priorities cannot be empty")
		}
		if c.AlertThresholds.Journalctl.Cooldown <= 0 {
			return fmt.Errorf("journalctl cooldown must be positive")
		}
	}

	// Validate Reboot configuration
	if c.AlertThresholds.Reboot.Enabled {
		if c.AlertThresholds.Reboot.UptimeThreshold <= 0 {
			return fmt.Errorf("reboot uptime threshold must be positive")
		}
	}

	// Validate Email configuration
	if c.Email.SMTPServer == "" {
		return fmt.Errorf("SMTP server cannot be empty")
	}
	if c.Email.SMTPPort <= 0 || c.Email.SMTPPort > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}
	if c.Email.From == "" {
		return fmt.Errorf("from email address cannot be empty")
	}
	if c.Email.To == "" {
		return fmt.Errorf("to email address cannot be empty")
	}

	return nil
}

// GetDiskPaths returns a comma-separated list of monitored disk paths
func (c *Config) GetDiskPaths() string {
	var paths []string
	for _, disk := range c.AlertThresholds.Disks {
		paths = append(paths, disk.Path)
	}
	return strings.Join(paths, ", ")
}
