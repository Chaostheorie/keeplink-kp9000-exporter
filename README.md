# Keeplink KP9000 Prometheus Exporter

[![Go](https://github.com/sergiosanche/keeplink-kp9000-exporter/actions/workflows/go.yml/badge.svg)](https://github.com/sergiosanche/keeplink-kp9000-exporter/actions)
[![Docker Image](https://img.shields.io/badge/docker-keeplink--kp9000--exporter-blue.svg)](https://hub.docker.com/r/sergiosanche/keeplink-kp9000-exporter)

Prometheus exporter for **Keeplink KP-9000-9XHML-X** (8x 2.5GbE + 1x 10G SFP+ managed switch).

Scrapes port statistics (TX/RX packets, link status, errors) from the web UI HTML (no SNMP support).

## Exposed Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `keeplink_port_tx_good_packets_total` | Counter | Total TX good packets per port | `{port="1"}` |
| `keeplink_port_rx_good_packets_total` | Counter | Total RX good packets per port | `{port="1"}` |
| `keeplink_port_link_up` | Gauge | Link status (1=up, 0=down) | `{port="1"}` |
| `keeplink_scrape_duration_seconds` | Gauge | Last scrape duration | - |
| `keeplink_scrape_errors_total` | Counter | Total scrape errors | - |

## Quick Start

```bash
# Binary
./keeplink-exporter --switch-ip=192.168.1.168 --listen=:9100

# Docker
docker run -p 9100:9100 \
  -e SWITCH_IP=192.168.1.168 \
  -e SWITCH_USER=admin \
  -e SWITCH_PASS=your_password \
  sergiosanche/keeplink-kp9000-exporter:latest


# Prometheus Configuration

```
scrape_configs:
  - job_name: 'keeplink-kp9000'
    static_configs:
      - targets: ['exporter-host:9100']
    scrape_interval: 30s
```

# Configuration Flags

--switch-ip=192.168.1.168    Switch IP address
--switch-user=admin          Web UI username (if login required)
--switch-pass=secret         Web UI password
--cookie=admin=abc123        Session cookie (login alternative)
--listen=:9100               Listen address
--timeout=10s                HTTP timeout

# Installation

```
git clone https://github.com/YOURUSERNAME/keeplink-kp9000-exporter
cd keeplink-kp9000-exporter
make build
./keeplink-exporter --switch-ip=192.168.1.168
```

# Docker

```
FROM golang:1.23-alpine AS builder
COPY . /src
WORKDIR /src
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/exporter .

FROM scratch
COPY --from=builder /bin/exporter /bin/exporter
EXPOSE 9100
ENTRYPOINT ["/bin/exporter"]
```

# Compatibility

✅ Keeplink KP-9000-9XHML-X (9 ports)

🔄 Other Keeplink models (testing needed)

❌ SNMP-enabled switches (use snmp_exporter)


