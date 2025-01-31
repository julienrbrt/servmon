package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

func monitorCPU(cfg *Config) {
	log.Printf("Monitoring CPU usage with threshold %.2f%% and cooldown %v", cfg.AlertThresholds.CPU.Threshold, cfg.AlertThresholds.CPU.Cooldown)

	alertCooldown := time.NewTimer(cfg.AlertThresholds.CPU.Cooldown)
	for {
		percent, err := cpu.Percent(time.Duration(1)*time.Second, false)
		if err != nil {
			log.Printf("Error getting CPU usage: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Average CPU usage across all cores
		var total float64
		for _, p := range percent {
			total += p
		}

		avg := total / float64(len(percent))

		if avg > cfg.AlertThresholds.CPU.Threshold {
			// Check if we're within the cooldown period
			select {
			case <-alertCooldown.C:
				// Cooldown expired, check again
				alertCooldown.Reset(cfg.AlertThresholds.CPU.Cooldown)
			default:
				// Within cooldown, skip alert
				time.Sleep(1 * time.Second)
				continue
			}

			err := sendEmail(fmt.Sprintf("CPU Usage Alert: %.2f%%", avg),
				fmt.Sprintf("CPU usage of %.2f%% has exceeded the threshold of %.2f%%", avg, cfg.AlertThresholds.CPU.Threshold), cfg)
			if err != nil {
				log.Printf("Error sending email: %v", err)
			}
		}

		time.Sleep(time.Duration(1) * time.Second)
	}
}

func monitorMemory(cfg *Config) {
	log.Printf("Monitoring memory usage with threshold %.2f%% and cooldown %v", cfg.AlertThresholds.Memory.Threshold, cfg.AlertThresholds.Memory.Cooldown)

	alertCooldown := time.NewTimer(cfg.AlertThresholds.Memory.Cooldown)
	for {
		vm, err := mem.VirtualMemory()
		if err != nil {
			log.Printf("Error getting memory usage: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		usedPercent := vm.UsedPercent

		if usedPercent > cfg.AlertThresholds.Memory.Threshold {
			// Check if we're within the cooldown period
			select {
			case <-alertCooldown.C:
				// Cooldown expired, check again
				alertCooldown.Reset(cfg.AlertThresholds.Memory.Cooldown)
			default:
				// Within cooldown, skip alert
				time.Sleep(1 * time.Second)
				continue
			}

			err := sendEmail(fmt.Sprintf("Memory Usage Alert: %.2f%%", usedPercent),
				fmt.Sprintf("Memory usage of %.2f%% has exceeded the threshold of %.2f%%", usedPercent, cfg.AlertThresholds.Memory.Threshold), cfg)
			if err != nil {
				log.Printf("Error sending email: %v", err)
			}
		}

		time.Sleep(time.Duration(1) * time.Second)
	}
}

func monitorHTTP(cfg *Config) {
	log.Printf("Monitoring HTTP checks (%s) with threshold %.2f%% and cooldown %v", cfg.AlertThresholds.HTTP.URL, cfg.AlertThresholds.HTTP.FailureThreshold, cfg.AlertThresholds.HTTP.Cooldown)

	alertCooldown := time.NewTimer(cfg.AlertThresholds.HTTP.Cooldown)
	client := &http.Client{
		Timeout: cfg.AlertThresholds.HTTP.Timeout,
	}

	for {
		// Wait for check interval
		time.Sleep(cfg.AlertThresholds.HTTP.CheckInterval)

		// Perform HTTP checks
		failureCount := 0
		for i := 0; i < cfg.AlertThresholds.HTTP.SampleRate; i++ {
			req, err := http.NewRequest("GET", cfg.AlertThresholds.HTTP.URL, nil)
			if err != nil {
				failureCount++
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.AlertThresholds.HTTP.Timeout)
			defer cancel()

			resp, err := client.Do(req.WithContext(ctx))
			if err != nil || resp.StatusCode >= 400 {
				failureCount++
			}
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
