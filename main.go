package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/julienrbrt/servmon/internal/alert"
	"github.com/julienrbrt/servmon/internal/config"
	"github.com/julienrbrt/servmon/internal/monitor"

	"github.com/spf13/cobra"
)

var (
	flagConfig = "config"
	cfgFile    string
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting user home directory: %v", err)
		os.Exit(1)
	}

	version, err := getVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting version: %v", err)
		version = "unknown"
	}

	rootCmd := &cobra.Command{
		Use:     "servmon",
		Short:   "Server monitoring tool with email alerts",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := cmd.Flags().GetString(flagConfig)
			if err != nil {
				return fmt.Errorf("error getting flag %s: %v", flagConfig, err)
			}

			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				cfg := config.Default()
				if err := cfg.Save(cfgFile); err != nil {
					return err
				}

				cmd.Println("✓ Configuration file generated at", cfgFile)
				cmd.Println("Please edit the configuration file and restart servmon")
				return nil
			} else if err != nil {
				return fmt.Errorf("error checking config file: %v", err)
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			// Show current system status
			if status, err := monitor.GetCurrentStatus(); err == nil {
				cmd.Println()
				cmd.Println(status)
				cmd.Println()
			}

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			// Create context for graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Initialize alerter
			emailAlerter := alert.NewEmailAlerter(alert.EmailConfig{
				SMTPServer: cfg.Email.SMTPServer,
				SMTPPort:   cfg.Email.SMTPPort,
				From:       cfg.Email.From,
				To:         cfg.Email.To,
				Username:   cfg.Email.Username,
				Password:   cfg.Email.Password,
			})

			// Check for recent reboot and send notification if needed
			if err := monitor.CheckRebootAndNotify(ctx, cfg, emailAlerter); err != nil {
				cmd.Printf("Warning: Failed to check reboot status: %v\n", err)
			}

			// Create monitor with alerter
			mon := monitor.New(cfg, emailAlerter)

			// Start monitoring
			mon.Start(ctx)

			cmd.Println()
			cmd.Println("✓ ServMon started successfully. Monitoring active.")
			cmd.Println("Monitoring in progress... Press Ctrl+C to stop.")
			cmd.Println()

			// Send startup notification
			go func() {
				hostname, err := os.Hostname()
				if err != nil {
					hostname = "unknown"
				}

				startupAlert := alert.NewAlert(
					alert.LevelInfo,
					fmt.Sprintf("Monitoring Started on %s", hostname),
					"ServMon has started successfully and is now actively monitoring your system.",
				)

				startupAlert.WithMetadata("cpu_threshold", fmt.Sprintf("%.1f%%", cfg.AlertThresholds.CPU.Threshold))
				startupAlert.WithMetadata("memory_threshold", fmt.Sprintf("%.1f%%", cfg.AlertThresholds.Memory.Threshold))
				startupAlert.WithMetadata("disk_paths", cfg.GetDiskPaths())

				if cfg.AlertThresholds.HTTP.URL != "" {
					startupAlert.WithMetadata("http_endpoint", cfg.AlertThresholds.HTTP.URL)
				}

				if err := emailAlerter.Send(ctx, startupAlert); err != nil {
					cmd.Printf("Warning: Failed to send startup notification: %v\n", err)
				} else {
					cmd.Println("✓ Startup notification sent successfully")
				}
			}()

			// Wait for shutdown signal
			sig := <-sigChan
			cmd.Printf("Received signal %v, shutting down gracefully...\n", sig)
			cancel()

			// Give goroutines time to clean up
			time.Sleep(1 * time.Second)
			cmd.Println("✓ ServMon stopped successfully")
			return nil
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&cfgFile, flagConfig, path.Join(homeDir, ".servmon.yaml"), "config file")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func getVersion() (string, error) {
	version, ok := debug.ReadBuildInfo()
	if !ok {
		return "", errors.New("failed to get version")
	}

	return strings.TrimSpace(version.Main.Version), nil
}
