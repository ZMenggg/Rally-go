# Configuration

## File Format

Rally uses a single YAML configuration file. By default it looks for `rally.yaml` in the current directory or `/etc/rally.yaml`.

## Schema

```yaml
bind: ":1080"
balance: roundrobin
log:
  level: info
vps:
  - name: us1
    type: hysteria2
    server: 192.0.2.1
    port: 23872
    password: "secret"
    down_mbps: 500
    up_mbps: 50
```

## Field Reference

### Top Level

| Field | Default | Description |
|---|---|---|
| `bind` | `:1080` | SOCKS5 listen address |
| `balance` | `roundrobin` | Load balancing algorithm |
| `log.level` | `info` | Log verbosity |

### VPS

| Field | Required | Description |
|---|---|---|
| `name` | YES | Unique identifier |
| `type` | YES | Protocol (`hysteria2` for now) |
| `server` | YES | VPS IP or hostname |
| `port` | YES | Hysteria2 port |
| `password` | YES | Auth credential |
| `sni` | No | TLS SNI (defaults to server) |
| `down_mbps` | No | Max download bandwidth (Mbps) |
| `up_mbps` | No | Max upload bandwidth (Mbps) |

## Multi-VPS Example

```yaml
bind: ":1080"
vps:
  - name: us-west
    type: hysteria2
    server: west.example.com
    port: 23872
    password: "p@ss1"
    down_mbps: 500
  - name: us-east
    type: hysteria2
    server: east.example.com
    port: 44705
    password: "p@ss2"
    down_mbps: 300
```