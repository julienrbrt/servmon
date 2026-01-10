package monitor

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/julienrbrt/servmon/internal/alert"
	"github.com/julienrbrt/servmon/internal/config"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// Monitor represents a system monitor
type Monitor struct {
	config  *config.Config
	alerter alert.Alerter
}

// New creates a new monitor
func New(cfg *config.Config, alerter alert.Alerter) *Monitor {
	return &Monitor{
		config:  cfg,
		alerter: alerter,
	}
}

// Start starts all monitoring goroutines
func (m *Monitor) Start(ctx context.Context) {
	log.Println("Starting monitoring services...")

	// Start CPU monitoring
	go m.MonitorCPU(ctx)

	// Start Memory monitoring
	go m.MonitorMemory(ctx)

	// Start Disk monitoring for each configured disk
	for _, diskCfg := range m.config.AlertThresholds.Disks {
		go m.MonitorDisk(ctx, diskCfg)
	}

	// Start HTTP monitoring if configured
	if m.config.AlertThresholds.HTTP.URL != "" {
		go m.MonitorHTTP(ctx)
	}

	// Start Journalctl monitoring if configured
	if m.config.AlertThresholds.Journalctl.Enabled {
		go m.MonitorJournalctl(ctx)
	}

	log.Println("✓ All monitoring services started successfully")
}

// MonitorCPU monitors CPU usage
func (m *Monitor) MonitorCPU(ctx context.Context) {
	cfg := m.config.AlertThresholds.CPU
	log.Printf("CPU Monitor: threshold=%.1f%%, interval=%v, cooldown=%v",
		cfg.Threshold, cfg.CheckInterval, cfg.Cooldown)

	// Initialize cooldown timer in expired state
	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C

	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("CPU monitor shutting down")
			return
		case <-ticker.C:
		}

		percent, err := cpu.Percent(cfg.Duration, false)
		if err != nil {
			log.Printf("Error getting CPU usage: %v", err)
			continue
		}

		// Calculate average CPU usage across all cores
		if len(percent) == 0 {
			log.Printf("CPU percentage returned empty array, skipping check")
			continue
		}

		var total float64
		for _, p := range percent {
			total += p
		}
		avg := total / float64(len(percent))

		// Check threshold
		if avg > cfg.Threshold {
			select {
			case <-alertCooldown.C:
				// Cooldown expired, send alert
				a := alert.NewAlert(
					alert.LevelWarning,
					fmt.Sprintf("High CPU Usage: %.2f%%", avg),
					fmt.Sprintf("CPU usage of %.2f%% has exceeded the threshold of %.2f%%", avg, cfg.Threshold),
				)
				a.WithMetadata("current_usage", fmt.Sprintf("%.2f%%", avg))
				a.WithMetadata("threshold", fmt.Sprintf("%.2f%%", cfg.Threshold))
				a.WithMetadata("duration", cfg.Duration.String())

				if err := m.alerter.Send(ctx, a); err != nil {
					log.Printf("Failed to send CPU alert: %v", err)
				}
				alertCooldown.Reset(cfg.Cooldown)
			default:
				// Within cooldown, skip alert
			}
		}
	}
}

// MonitorMemory monitors memory usage
func (m *Monitor) MonitorMemory(ctx context.Context) {
	cfg := m.config.AlertThresholds.Memory
	log.Printf("Memory Monitor: threshold=%.1f%%, interval=%v, cooldown=%v",
		cfg.Threshold, cfg.CheckInterval, cfg.Cooldown)

	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C

	ticker := time.NewTicker(cfg.CheckInterval)
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

		if usedPercent > cfg.Threshold {
			select {
			case <-alertCooldown.C:
				// Cooldown expired, send alert
				a := alert.NewAlert(
					alert.LevelWarning,
					fmt.Sprintf("High Memory Usage: %.2f%%", usedPercent),
					fmt.Sprintf("Memory usage of %.2f%% has exceeded the threshold of %.2f%%", usedPercent, cfg.Threshold),
				)
				a.WithMetadata("current_usage", fmt.Sprintf("%.2f%%", usedPercent))
				a.WithMetadata("threshold", fmt.Sprintf("%.2f%%", cfg.Threshold))
				a.WithMetadata("used", fmt.Sprintf("%.2f GB", float64(vm.Used)/(1024*1024*1024)))
				a.WithMetadata("total", fmt.Sprintf("%.2f GB", float64(vm.Total)/(1024*1024*1024)))
				a.WithMetadata("available", fmt.Sprintf("%.2f GB", float64(vm.Available)/(1024*1024*1024)))

				if err := m.alerter.Send(ctx, a); err != nil {
					log.Printf("Failed to send memory alert: %v", err)
				}
				alertCooldown.Reset(cfg.Cooldown)
			default:
				// Within cooldown, skip alert
			}
		}
	}
}

// MonitorDisk monitors disk usage
func (m *Monitor) MonitorDisk(ctx context.Context, diskCfg config.DiskConfig) {
	log.Printf("📊 Disk Monitor [%s]: threshold=%.1f%%, interval=%v, cooldown=%v",
		diskCfg.Path, diskCfg.Threshold, diskCfg.CheckInterval, diskCfg.Cooldown)

	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C

	ticker := time.NewTicker(diskCfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Disk monitor [%s] shutting down", diskCfg.Path)
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
			select {
			case <-alertCooldown.C:
				// Cooldown expired, send alert
				a := alert.NewAlert(
					alert.LevelWarning,
					fmt.Sprintf("High Disk Usage: %s %.2f%%", diskCfg.Path, usedPercent),
					fmt.Sprintf("Disk usage for %s of %.2f%% has exceeded the threshold of %.2f%%", diskCfg.Path, usedPercent, diskCfg.Threshold),
				)
				a.WithMetadata("path", diskCfg.Path)
				a.WithMetadata("current_usage", fmt.Sprintf("%.2f%%", usedPercent))
				a.WithMetadata("threshold", fmt.Sprintf("%.2f%%", diskCfg.Threshold))
				a.WithMetadata("used", fmt.Sprintf("%.2f GB", float64(usage.Used)/(1024*1024*1024)))
				a.WithMetadata("total", fmt.Sprintf("%.2f GB", float64(usage.Total)/(1024*1024*1024)))
				a.WithMetadata("free", fmt.Sprintf("%.2f GB", float64(usage.Free)/(1024*1024*1024)))

				if err := m.alerter.Send(ctx, a); err != nil {
					log.Printf("Failed to send disk alert: %v", err)
				}
				alertCooldown.Reset(diskCfg.Cooldown)
			default:
				// Within cooldown, skip alert
			}
		}
	}
}

// MonitorHTTP monitors HTTP endpoint health
func (m *Monitor) MonitorHTTP(ctx context.Context) {
	cfg := m.config.AlertThresholds.HTTP
	log.Printf("📊 HTTP Monitor [%s]: failure_threshold=%.1f%%, interval=%v, cooldown=%v",
		cfg.URL, cfg.FailureThreshold, cfg.CheckInterval, cfg.Cooldown)

	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("HTTP monitor shutting down")
			return
		case <-time.After(cfg.CheckInterval):
		}

		// Perform HTTP checks in batch to calculate failure rate
		failureCount := 0
		for i := 0; i < cfg.SampleRate; i++ {
			req, err := http.NewRequest("GET", cfg.URL, nil)
			if err != nil {
				failureCount++
				continue
			}

			reqCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)

			resp, err := client.Do(req.WithContext(reqCtx))
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
		failureRate := (float64(failureCount) / float64(cfg.SampleRate)) * 100

		if failureRate > cfg.FailureThreshold {
			select {
			case <-alertCooldown.C:
				// Cooldown expired, send alert
				level := alert.LevelWarning
				if failureRate >= 50 {
					level = alert.LevelCritical
				}

				a := alert.NewAlert(
					level,
					fmt.Sprintf("HTTP Health Check Failed: %.2f%%", failureRate),
					fmt.Sprintf("HTTP endpoint %s has a failure rate of %.2f%% (threshold: %.2f%%)", cfg.URL, failureRate, cfg.FailureThreshold),
				)
				a.WithMetadata("url", cfg.URL)
				a.WithMetadata("failure_rate", fmt.Sprintf("%.2f%%", failureRate))
				a.WithMetadata("threshold", fmt.Sprintf("%.2f%%", cfg.FailureThreshold))
				a.WithMetadata("failures", fmt.Sprintf("%d/%d", failureCount, cfg.SampleRate))
				a.WithMetadata("sample_rate", cfg.SampleRate)

				if err := m.alerter.Send(ctx, a); err != nil {
					log.Printf("Failed to send HTTP alert: %v", err)
				}
				alertCooldown.Reset(cfg.Cooldown)
			default:
				// Within cooldown, skip alert
			}
		}
	}
}

// GetCurrentStatus returns the current system status
func GetCurrentStatus() (string, error) {
	// Get CPU usage
	cpuPercent, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		return "", fmt.Errorf("failed to get CPU usage: %w", err)
	}
	var cpuTotal float64
	for _, p := range cpuPercent {
		cpuTotal += p
	}
	cpuAvg := cpuTotal / float64(len(cpuPercent))

	// Get memory usage
	vm, err := mem.VirtualMemory()
	if err != nil {
		return "", fmt.Errorf("failed to get memory usage: %w", err)
	}

	// Get root disk usage
	rootDisk, err := disk.Usage("/")
	if err != nil {
		return "", fmt.Errorf("failed to get disk usage: %w", err)
	}

	status := fmt.Sprintf(
		"Current System Status:\n"+
			"CPU: %.2f%%\n"+
			"Memory: %.2f%% (%.2f GB used / %.2f GB total)\n"+
			"Disk: %.2f%% (%.2f GB used / %.2f GB total)",
		cpuAvg,
		vm.UsedPercent,
		float64(vm.Used)/(1024*1024*1024),
		float64(vm.Total)/(1024*1024*1024),
		rootDisk.UsedPercent,
		float64(rootDisk.Used)/(1024*1024*1024),
		float64(rootDisk.Total)/(1024*1024*1024),
	)

	return status, nil
}

// MonitorJournalctl monitors systemd journal logs for errors and critical messages
func (m *Monitor) MonitorJournalctl(ctx context.Context) {
	cfg := m.config.AlertThresholds.Journalctl
	log.Printf("📊 Journalctl Monitor: error_threshold=%d, interval=%v, lookback=%v, priority=%v, cooldown=%v",
		cfg.ErrorThreshold, cfg.CheckInterval, cfg.LookbackPeriod, cfg.Priority, cfg.Cooldown)

	alertCooldown := time.NewTimer(0)
	<-alertCooldown.C

	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Journalctl monitor shutting down")
			return
		case <-ticker.C:
		}

		// Build journalctl command with priority filters
		sinceArg := fmt.Sprintf("%dm ago", int(cfg.LookbackPeriod.Minutes()))

		cmd := exec.CommandContext(ctx, "journalctl", "-x", "-e",
			"--since", sinceArg,
			"-p", cfg.Priority,
			"--no-pager",
			"-o", "short-precise")

		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Error running journalctl: %v. Output: %s", err, string(output))
			continue
		}

		// Parse the output
		logs := string(output)
		if logs == "" || strings.TrimSpace(logs) == "" {
			// No errors found, which is good
			continue
		}

		// Count and aggregate errors by message pattern
		lines := strings.Split(logs, "\n")
		errorCount := 0
		errorPatterns := make(map[string]int)
		criticalCount := 0

		// Regex to extract the main error message (after the process name)
		messageRegex := regexp.MustCompile(`\]: (.+)$`)

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "--") {
				continue
			}

			errorCount++

			// Check for critical level
			if strings.Contains(strings.ToLower(line), "crit") ||
				strings.Contains(strings.ToLower(line), "alert") ||
				strings.Contains(strings.ToLower(line), "emerg") {
				criticalCount++
			}

			// Extract and aggregate error patterns
			matches := messageRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				message := matches[1]
				// Normalize the message (remove specific IDs, paths, etc.)
				normalized := normalizeErrorMessage(message)
				errorPatterns[normalized]++
			}
		}

		// Check if error count exceeds threshold
		if errorCount > cfg.ErrorThreshold {
			select {
			case <-alertCooldown.C:
				// Cooldown expired, send alert
				level := alert.LevelWarning
				if criticalCount > 0 {
					level = alert.LevelCritical
				}

				// Build top error patterns for the alert
				topErrors := getTopErrors(errorPatterns, 5)

				title := fmt.Sprintf("Journal Log Errors Detected: %d errors", errorCount)
				message := fmt.Sprintf("Found %d error/critical messages in journalctl logs (threshold: %d). ",
					errorCount, cfg.ErrorThreshold)

				if criticalCount > 0 {
					message += fmt.Sprintf("%d critical-level messages detected. ", criticalCount)
				}

				message += "Review system logs for details."

				a := alert.NewAlert(level, title, message)
				a.WithMetadata("error_count", errorCount)
				a.WithMetadata("critical_count", criticalCount)
				a.WithMetadata("threshold", cfg.ErrorThreshold)
				a.WithMetadata("lookback_period", cfg.LookbackPeriod.String())
				a.WithMetadata("priority", cfg.Priority)

				if len(topErrors) > 0 {
					a.WithMetadata("top_errors", strings.Join(topErrors, " | "))
				}

				if err := m.alerter.Send(ctx, a); err != nil {
					log.Printf("Failed to send journalctl alert: %v", err)
				}
				alertCooldown.Reset(cfg.Cooldown)
			default:
				// Within cooldown, skip alert
			}
		}
	}
}

// normalizeErrorMessage removes specific details from error messages to help aggregate similar errors
func normalizeErrorMessage(msg string) string {
	// Remove PIDs
	re := regexp.MustCompile(`\b\d{3,}\b`)
	msg = re.ReplaceAllString(msg, "[PID]")

	// Remove file paths
	re = regexp.MustCompile(`/[\w/.-]+`)
	msg = re.ReplaceAllString(msg, "[PATH]")

	// Remove timestamps
	re = regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)
	msg = re.ReplaceAllString(msg, "[TIME]")

	// Remove hex addresses
	re = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	msg = re.ReplaceAllString(msg, "[ADDR]")

	// Truncate to first 100 chars
	if len(msg) > 100 {
		msg = msg[:100] + "..."
	}

	return msg
}

// getTopErrors returns the top N most frequent error patterns
func getTopErrors(patterns map[string]int, n int) []string {
	type errorCount struct {
		pattern string
		count   int
	}

	var errors []errorCount
	for pattern, count := range patterns {
		errors = append(errors, errorCount{pattern, count})
	}

	// Simple bubble sort for small lists
	for i := 0; i < len(errors); i++ {
		for j := i + 1; j < len(errors); j++ {
			if errors[j].count > errors[i].count {
				errors[i], errors[j] = errors[j], errors[i]
			}
		}
	}

	// Get top N
	var result []string
	limit := n
	if len(errors) < limit {
		limit = len(errors)
	}

	for i := 0; i < limit; i++ {
		result = append(result, fmt.Sprintf("%s (×%d)", errors[i].pattern, errors[i].count))
	}

	return result
}

// CheckRebootAndNotify checks if the system recently rebooted and sends notification
func CheckRebootAndNotify(ctx context.Context, cfg *config.Config, alerter alert.Alerter) error {
	if !cfg.AlertThresholds.Reboot.Enabled {
		return nil
	}

	// Get system boot time
	bootTime, err := host.BootTime()
	if err != nil {
		return fmt.Errorf("failed to get boot time: %w", err)
	}

	// Calculate uptime
	uptime := time.Since(time.Unix(int64(bootTime), 0))

	log.Printf("System uptime: %v (reboot threshold: %v)",
		uptime.Round(time.Second), cfg.AlertThresholds.Reboot.UptimeThreshold)

	// Check if uptime is less than threshold (indicating recent reboot)
	if uptime < cfg.AlertThresholds.Reboot.UptimeThreshold {
		log.Printf("Recent reboot detected - system uptime is %v", uptime.Round(time.Second))

		bootTimeFormatted := time.Unix(int64(bootTime), 0).Format(time.RFC1123)

		a := alert.NewAlert(
			alert.LevelInfo,
			"System Reboot Detected",
			fmt.Sprintf("The system was recently rebooted. Current uptime: %s. Boot time: %s",
				formatDuration(uptime), bootTimeFormatted),
		)

		a.WithMetadata("uptime", formatDuration(uptime))
		a.WithMetadata("boot_time", bootTimeFormatted)
		a.WithMetadata("uptime_threshold", cfg.AlertThresholds.Reboot.UptimeThreshold.String())

		// Get system info
		if info, err := host.Info(); err == nil {
			a.WithMetadata("os", info.OS)
			a.WithMetadata("platform", info.Platform)
			a.WithMetadata("platform_version", info.PlatformVersion)
			a.WithMetadata("kernel_version", info.KernelVersion)
		}

		if err := alerter.Send(ctx, a); err != nil {
			return fmt.Errorf("failed to send reboot notification: %w", err)
		}

		log.Println("✓ Reboot notification sent successfully")
	}

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour

	hours := d / time.Hour
	d -= hours * time.Hour

	minutes := d / time.Minute
	d -= minutes * time.Minute

	seconds := d / time.Second

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
