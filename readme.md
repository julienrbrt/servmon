# Servmon

KISS server monitoring tool with email alerts.
For those who want to keep it simple instead of using Prometheus, Grafana, and Alertmanager.
It uses the awesome [gopsutil](https://github.com/shirou/gopsutil) library to get system metrics.

Monitors:

- [x] CPU
- [x] Memory
- [x] HTTP Health check
- [x] Disk Usage
- [ ] Disk Write/Read
- [ ] Docker

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
