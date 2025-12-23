package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagConfig  = "config"
	flagDaemon  = "daemon"
	cfgFile     string
	runAsDaemon bool
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
		Short:   "KISS server monitoring tool with email alerts",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			runAsDaemon, err := cmd.Flags().GetBool(flagDaemon)
			if err != nil {
				return fmt.Errorf("error getting flag %s: %v", flagDaemon, err)
			}

			if runAsDaemon {
				pid, err := runAsDaemonProcess()
				if err != nil {
					return err
				}

				cmd.Println("Running as daemon with PID", pid)
				return nil
			}

			cfgPath, err := cmd.Flags().GetString(flagConfig)
			if err != nil {
				return fmt.Errorf("error getting flag %s: %v", flagConfig, err)
			}

			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				cfg := defaultConfig()
				if err := cfg.Save(cfgFile); err != nil {
					return err
				}

				cmd.Println("Configuration file generated at", cfgFile)
				return nil
			} else if err != nil {
				return fmt.Errorf("error checking config file: %v", err)
			}

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			// Create context for graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Start monitoring goroutines
			go monitorCPU(ctx, cfg)
			go monitorMemory(ctx, cfg)

			for _, diskCfg := range cfg.AlertThresholds.Disks {
				go monitorDisk(ctx, cfg, diskCfg)
			}

			if cfg.AlertThresholds.HTTP.URL != "" {
				go monitorHTTP(ctx, cfg)
			}

			cmd.Println("Servmon started successfully. Monitoring active.")
			cmd.Println("Press Ctrl+C to stop.")

			// Send email notification that monitoring is now active
			go func() {
				hostname, err := os.Hostname()
				if err != nil {
					hostname = "unknown"
				}

				subject := fmt.Sprintf("Monitoring Active on %s", hostname)
				body := fmt.Sprintf("ServMon has started successfully and is now actively monitoring:\n\n"+
					"- CPU threshold: %.1f%%\n"+
					"- Memory threshold: %.1f%%\n"+
					"- Disk paths: %s\n",
					cfg.AlertThresholds.CPU.Threshold,
					cfg.AlertThresholds.Memory.Threshold,
					getDiskPaths(cfg))

				if cfg.AlertThresholds.HTTP.URL != "" {
					body += fmt.Sprintf("- HTTP endpoint: %s\n", cfg.AlertThresholds.HTTP.URL)
				}

				body += fmt.Sprintf("\nMonitoring started at: %s", time.Now().Format(time.RFC1123))

				if err := sendEmail(subject, body, cfg); err != nil {
					cmd.Printf("Warning: Failed to send monitoring active notification: %v\n", err)
				} else {
					cmd.Println("Monitoring active notification sent successfully.")
				}
			}()

			// Wait for shutdown signal
			sig := <-sigChan
			cmd.Printf("\nReceived signal %v, shutting down gracefully...\n", sig)
			cancel()
			return nil
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&cfgFile, flagConfig, path.Join(homeDir, ".servmon.yaml"), "config file")
	rootCmd.PersistentFlags().BoolVarP(&runAsDaemon, flagDaemon, "d", false, "run as daemon")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func runAsDaemonProcess() (int, error) {
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" {
		var args []string
		for _, a := range os.Args[1:] {
			if a != "-d" && a != "--daemon" {
				args = append(args, a)
			}
		}

		cmd := exec.Command(os.Args[0], args...)
		cmd.Stdout = os.NewFile(3, "log.out")
		cmd.Stderr = os.NewFile(4, "log.err")
		cmd.Stdin = os.NewFile(3, "log.in")

		if err := cmd.Start(); err != nil {
			return 0, fmt.Errorf("error starting as daemon: %v", err)
		}

		pid := cmd.Process.Pid

		// Detach the process
		err := cmd.Process.Release()
		if err != nil {
			return 0, fmt.Errorf("error detaching process: %v", err)
		}

		return pid, nil
	}

	return 0, fmt.Errorf("daemon mode is only supported on Linux and FreeBSD, not on %s", runtime.GOOS)
}

func getVersion() (string, error) {
	version, ok := debug.ReadBuildInfo()
	if !ok {
		return "", errors.New("failed to get version")
	}

	return strings.TrimSpace(version.Main.Version), nil
}

// getDiskPaths returns a comma-separated list of monitored disk paths
func getDiskPaths(cfg *Config) string {
	var paths []string
	for _, disk := range cfg.AlertThresholds.Disks {
		paths = append(paths, disk.Path)
	}
	return strings.Join(paths, ", ")
}
