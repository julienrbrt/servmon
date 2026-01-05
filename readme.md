# ServMon

KISS (Keep It Simple, Stupid) server monitoring tool with email alerts.

For those who want to keep it simple instead of using complex setups like Prometheus, Grafana, and Alertmanager.

## Features

- **CPU & Memory Monitoring** - Alert on high usage
- **Disk Monitoring** - Monitor multiple partitions
- **HTTP Health Checks** - Monitor endpoint availability
- **Journalctl Monitoring** - Scan systemd logs for errors
- **Reboot Detection** - Email notification on system reboot
- **Email Alerts** - Rich formatting with severity levels
- **Smart Cooldowns** - Prevent alert spam

## Installation

```bash
go install github.com/julienrbrt/servmon@latest
```

## Configuration

Edit `~/.servmon.yaml` or `/etc/servmon/config.yaml`:

```yaml
alert_thresholds:
  cpu:
    threshold: 90
    duration: 5m0s
    cooldown: 30m0s
    check_interval: 10s

  memory:
    threshold: 80
    cooldown: 30m0s
    check_interval: 10s

  disks:
    - path: /
      threshold: 90
      cooldown: 4h0m0s
      check_interval: 1m0s

  http:
    url: http://localhost:8080/health
    timeout: 5s
    sample_rate: 10
    failure_threshold: 20
    check_interval: 1m0s
    cooldown: 15m0s

  journalctl:
    enabled: true
    check_interval: 5m0s
    lookback_period: 5m0s
    error_threshold: 10
    priorities:
      - err
      - crit
      - alert
      - emerg
    cooldown: 30m0s

  reboot:
    enabled: true
    uptime_threshold: 10m0s

email:
  smtp_server: smtp.gmail.com
  smtp_port: 587
  from: alerts@yourdomain.com
  to: admin@yourdomain.com
  username: alerts@yourdomain.com
  password: your-app-password
```

## Usage

### Run Manually

```bash
servmon --config ~/.servmon.yaml
```

### Systemd Service

```bash
# Start
sudo systemctl start servmon

# Enable auto-start
sudo systemctl enable servmon

# View logs
sudo journalctl -u servmon -f

# Check status
sudo systemctl status servmon
```

## License

[MIT](license).
