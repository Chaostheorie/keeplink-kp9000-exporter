# Keeplink KP9000 Prometheus Exporter

[![Go](https://github.com/sergiosanche/keeplink-kp9000-exporter/actions/workflows/go.yml/badge.svg)](https://github.com/sergiosanche/keeplink-kp9000-exporter/actions)
[![Docker Image](https://img.shields.io/badge/docker-keeplink--kp9000--exporter-blue.svg)](https://hub.docker.com/r/sergiosanche/keeplink-kp9000-exporter)

Prometheus exporter for **Keeplink KP-9000-9XHML-X** (8x 2.5GbE + 1x 10G SFP+ managed switch).

Scrapes port statistics (TX/RX packets, link status, errors) from the web UI HTML (no SNMP support).

## Exposed Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `keeplink_port_link_up` | Gauge | Link status (1=up, 0=down) | `{port="1"}` |
| `keeplink_port_rx_good_packets_total` | Counter | Total RX good packets per port | `{port="1"}` |
| `keeplink_port_tx_good_packets_total` | Counter | Total TX good packets per port | `{port="1"}` |
| `keeplink_port_speed_actual_mbps` | Gauage | Actual Mbps per port | `{port="1000"}` |

## Quick Start

```bash
# Binary
./exporter-keeplink --ip=192.168.1.168 --cookie="admin=f6fdffe48c908deb0f4c3bd36c032e72" --listen=:9101

# In a different window
curl localhost:9101/metrics -s |grep keeplink
```

```bash
# Docker
docker run -p 9101:9101 \
  -e SWITCH_IP=192.168.1.168 \
  -e SWITCH_USER=admin \
  -e SWITCH_COOKIE="admin=f6fdffe48c908deb0f4c3bd36c032e72" \
  sergiosanche/keeplink-kp9000-exporter:latest
```

# Prometheus Configuration

```
scrape_configs:
  - job_name: 'keeplink-kp9000'
    static_configs:
      - targets: ['exporter-host:9101']
    scrape_interval: 30s
```

# Configuration Flags

--ip=192.168.1.168                                 Switch IP address
--cookie="admin=f6fdffe48c908deb0f4c3bd36c032e72"  Session cookie
--listen=:9101                                     Listen address
--password=''                                      Admin user password (initial username: admin)
--username=''                                      Admin user username (initial username: password)

Specifying a cookie will supseed the username/password.

# Installation

```
git clone https://github.com/sergiosanche/keeplink-kp9000-exporter
cd keeplink-kp9000-exporter
make build
./bin/exporter-keeplink --ip=192.168.1.168 --cookie="admin=f6fdffe48c908deb0f4c3bd36c032e72" --listen=:9101
```

# Docker

```
FROM golang:1.23-alpine AS builder
COPY . /src
WORKDIR /src
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/exporter .

FROM scratch
COPY --from=builder /bin/exporter-keeplink /bin/exporter-keeplink
EXPOSE 9101
ENTRYPOINT ["/bin/exporter-keeplink"]
```

# Compatibility

✅ Keeplink KP-9000-9XHML-X (9 ports)

🔄 Other Keeplink models (testing needed)

❌ SNMP-enabled switches (use snmp_exporter)
