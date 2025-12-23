package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

func monitorCPU(ctx context.Context, cfg *Config) {
	log.Printf("Monitoring CPU usage with threshold %.2f%%, check interval %v, and cooldown %v",
		cfg.AlertThresholds.CPU.Threshold, cfg.AlertThresholds.CPU.CheckInterval, cfg.AlertThresholds.CPU.Cooldown)

	// Initialize cooldown timer in expired state so the first alert can fire immediately.
	// We create a timer with 0 duration and drain it right away, so the select case
	// <-alertCooldown.C will succeed on the first threshold breach.
	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C

	ticker := time.NewTicker(cfg.AlertThresholds.CPU.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("CPU monitor shutting down")
			return
		case <-ticker.C:
		}
		percent, err := cpu.Percent(cfg.AlertThresholds.CPU.Duration, false)
		if err != nil {
			log.Printf("Error getting CPU usage: %v", err)
			continue
		}

		// Average CPU usage across all cores
		var total float64
		for _, p := range percent {
			total += p
		}

		// Safety check: prevent division by zero
		if len(percent) == 0 {
			log.Printf("Warning: CPU percentage returned empty array, skipping check")
			continue
		}
		avg := total / float64(len(percent))

		if avg > cfg.AlertThresholds.CPU.Threshold {
			// Check if we're within the cooldown period using non-blocking select.
			// If the timer has expired, we can send an alert and reset the timer.
			// If not, we skip the alert to prevent spam.
			select {
			case <-alertCooldown.C:
				// Cooldown expired, send alert
				err := sendEmail(fmt.Sprintf("CPU Usage Alert: %.2f%%", avg),
					fmt.Sprintf("CPU usage of %.2f%% has exceeded the threshold of %.2f%%", avg, cfg.AlertThresholds.CPU.Threshold), cfg)
				if err != nil {
					log.Printf("Error sending email: %v", err)
				}
				// Reset timer to start a new cooldown period
				alertCooldown.Reset(cfg.AlertThresholds.CPU.Cooldown)
			default:
				// Within cooldown, skip alert
			}
		}
	}
}

func monitorMemory(ctx context.Context, cfg *Config) {
	log.Printf("Monitoring memory usage with threshold %.2f%%, check interval %v, and cooldown %v",
		cfg.AlertThresholds.Memory.Threshold, cfg.AlertThresholds.Memory.CheckInterval, cfg.AlertThresholds.Memory.Cooldown)

	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C // Drain the initial timer immediately so first alert can fire

	ticker := time.NewTicker(cfg.AlertThresholds.Memory.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Memory monitor shutting down")
			return
		case <-ticker.C:
		}
		vm, err := mem.VirtualMemory()
		if err != nil {
			log.Printf("Error getting memory usage: %v", err)
			continue
		}

		usedPercent := vm.UsedPercent

		if usedPercent > cfg.AlertThresholds.Memory.Threshold {
			// Check if we're within the cooldown period
			select {
			case <-alertCooldown.C:
				// Cooldown expired, send alert
				err := sendEmail(fmt.Sprintf("Memory Usage Alert: %.2f%%", usedPercent),
					fmt.Sprintf("Memory usage of %.2f%% has exceeded the threshold of %.2f%%", usedPercent, cfg.AlertThresholds.Memory.Threshold), cfg)
				if err != nil {
					log.Printf("Error sending email: %v", err)
				}
				alertCooldown.Reset(cfg.AlertThresholds.Memory.Cooldown)
			default:
				// Within cooldown, skip alert
			}
		}
	}
}

func monitorDisk(ctx context.Context, cfg *Config, diskCfg DiskConfig) {
	log.Printf("Monitoring disk %s usage with threshold %.2f%%, check interval %v, and cooldown %v",
		diskCfg.Path, diskCfg.Threshold, diskCfg.CheckInterval, diskCfg.Cooldown)

	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C // Drain the initial timer immediately so first alert can fire

	ticker := time.NewTicker(diskCfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Disk monitor for %s shutting down\n", diskCfg.Path)
			return
		case <-ticker.C:
		}
		usage, err := disk.Usage(diskCfg.Path)
		if err != nil {
			log.Printf("Error getting disk usage for %s: %v", diskCfg.Path, err)
			continue
		}

		usedPercent := usage.UsedPercent
		if usedPercent > diskCfg.Threshold {
			// Check if we're within the cooldown period
			select {
			case <-alertCooldown.C:
				// Cooldown expired, send alert
				err := sendEmail(fmt.Sprintf("Disk Usage Alert: %s %.2f%%", diskCfg.Path, usedPercent),
					fmt.Sprintf("Disk usage for %s of %.2f%% has exceeded the threshold of %.2f%%", diskCfg.Path, usedPercent, diskCfg.Threshold), cfg)
				if err != nil {
					log.Printf("Error sending email: %v", err)
				}
				alertCooldown.Reset(diskCfg.Cooldown)
			default:
				// Within cooldown, skip alert
			}
		}
	}
}

func monitorHTTP(ctx context.Context, cfg *Config) {
	log.Printf("Monitoring HTTP checks (%s) with threshold %.2f%% and cooldown %v", cfg.AlertThresholds.HTTP.URL, cfg.AlertThresholds.HTTP.FailureThreshold, cfg.AlertThresholds.HTTP.Cooldown)

	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C // Drain the initial timer immediately so first alert can fire
	client := &http.Client{
		Timeout: cfg.AlertThresholds.HTTP.Timeout,
	}

	for {
		// Wait for check interval or context cancellation
		select {
		case <-ctx.Done():
			log.Println("HTTP monitor shutting down")
			return
		case <-time.After(cfg.AlertThresholds.HTTP.CheckInterval):
		}

		// Perform HTTP checks in batch to calculate failure rate.
		// We make sample_rate number of requests and track how many fail.
		failureCount := 0
		for i := 0; i < cfg.AlertThresholds.HTTP.SampleRate; i++ {
			req, err := http.NewRequest("GET", cfg.AlertThresholds.HTTP.URL, nil)
			if err != nil {
				failureCount++
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.AlertThresholds.HTTP.Timeout)

			resp, err := client.Do(req.WithContext(ctx))
			if err != nil {
				failureCount++
			} else {
				if resp.StatusCode >= 400 {
					failureCount++
				}
				resp.Body.Close()
			}

			cancel()
		}

		// Calculate failure rate
		failureRate := (float64(failureCount) / float64(cfg.AlertThresholds.HTTP.SampleRate)) * 100
		if failureRate > cfg.AlertThresholds.HTTP.FailureThreshold {
			// Check if we're within the cooldown period
			select {
			case <-alertCooldown.C:
				// Cooldown expired, check again
				alertCooldown.Reset(cfg.AlertThresholds.HTTP.Cooldown)
			default:
				// Within cooldown, skip alert
				continue
			}

			err := sendEmail(fmt.Sprintf("HTTP Failure Alert: %.2f%%", failureRate),
				fmt.Sprintf("HTTP failure rate of %.2f%% has exceeded the threshold of %.2f%%", failureRate, cfg.AlertThresholds.HTTP.FailureThreshold), cfg)
			if err != nil {
				log.Printf("Error sending email: %v", err)
			}
		}
	}
}
