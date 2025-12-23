# Servmon

KISS (Keep It Simple, Stupid) server monitoring tool with email alerts.

For those who want to keep it simple instead of using complex setups like Prometheus, Grafana, and Alertmanager.
It uses the awesome [gopsutil](https://github.com/shirou/gopsutil) library to get system metrics.

## Features

- [x] **CPU Monitoring** - Monitor CPU usage with configurable thresholds and duration
- [x] **Memory Monitoring** - Track memory usage with percentage-based alerts
- [x] **Disk Monitoring** - Monitor multiple disk partitions independently
- [x] **HTTP Health Checks** - Periodic health checks with failure rate monitoring
- [x] **Email Alerts** - SMTP-based email notifications with configurable cooldowns
- [x] **Graceful Shutdown** - Clean shutdown on SIGTERM/SIGINT
- [x] **Config Validation** - Automatic validation of configuration parameters
- [ ] Disk Write/Read performance monitoring

## Installation

### Go

```bash
go install github.com/julienrbrt/servmon@latest
```

### Docker

```bash
docker build -t servmon .
```

## How to use

### Go

```bash
servmon --help
```

### Docker

```bash
# Create config directory
mkdir -p config
cp .servmon.example.yaml config/.servmon.yaml
# Edit config/.servmon.yaml with your settings

# Run
docker run -d \
  --name servmon \
  --restart unless-stopped \
  -v $(pwd)/config/.servmon.yaml:/root/.servmon.yaml:ro \
  servmon
```
