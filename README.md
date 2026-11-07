# Keeplink KP9000 Prometheus Exporter (Fork)

Prometheus exporter for **Keeplink KP-9000-9XHML-X** (8x 2.5GbE + 1x 10G SFP+ managed switch).
Fork from [github:sergiosanche/keeplink-kp9000-exporter](https://github.com/sergiosanche/keeplink-kp9000-exporter) with support for multiple switches and username/password authentication.

Scrapes port statistics (TX/RX packets, link status, errors) from the web UI HTML (no SNMP support).

## Exposed Metrics

| Metric                                | Type    | Description                    | Labels          |
| ------------------------------------- | ------- | ------------------------------ | --------------- |
| `keeplink_port_link_up`               | Gauge   | Link status (1=up, 0=down)     | `{port="1"}`    |
| `keeplink_port_rx_good_packets_total` | Counter | Total RX good packets per port | `{port="1"}`    |
| `keeplink_port_tx_good_packets_total` | Counter | Total TX good packets per port | `{port="1"}`    |
| `keeplink_port_speed_actual_mbps`     | Gauage  | Actual Mbps per port           | `{port="1000"}` |

## Quick Start

```bash
# Binary
./exporter-keeplink --ip=192.168.1.168 --cookie="admin=f6fdffe48c908deb0f4c3bd36c032e72" --listen=:9101

# In a different window
curl localhost:9101/metrics -s |grep keeplink
```

## NixOS

Import the flake's `nixosModules.default` and configure the switches to scrape:

```nix
{
  imports = [ inputs.keeplink-exporter.nixosModules.default ];

  services.keeplink-exporter = {
    enable = true;
    config.switches = [
      {
        switchIP = "192.168.1.168";
        usernameFile = "/run/secrets/keeplink-username";
        passwordFile = "/run/secrets/keeplink-password";
        timeout = "10s";
      }
    ];
  };
}
```

The module writes the scrape configuration as YAML and uses systemd credentials
to make each username and password file available to the service's dynamic user.
Use runtime path strings for these files. The optional timeout defaults to `"0s"`
(no timeout). Metrics listen on `127.0.0.1:8192`.
