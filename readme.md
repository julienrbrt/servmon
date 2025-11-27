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

```bash
go install github.com/julienrbrt/servmon@latest
```

## How to use

```bash
servmon --help
```
