# Rally-go ⚡

**Rally-go** — Fuse multiple VPS proxy connections into a single high-speed pipe,
aggregating their bandwidth for faster downloads.

Single Go binary, MIT license, Docker / bare-metal deployable.

[中文](README.md) | English

---

## Features

- **Multi-Protocol** — Hysteria2, SOCKS5, Shadowsocks, Trojan, VLESS
- **Bandwidth Aggregation** — Connection-level roundrobin / leastconn load balancing
- **Web Dashboard** — Visual node management, real-time traffic monitoring, live logs (i18n)
- **Hot Reload** — `SIGHUP` triggers config reload without restart
- **Live Monitoring** — Per-node throughput, aggregate speed, cumulative traffic
- **Node Toggle** — Enable/disable nodes from dashboard with one click
- **Health Check** — Automatic background health checks, removes dead nodes from rotation

## Architecture

```text
App (any SOCKS5-compatible software)
    │  socks5://127.0.0.1:1080
    ▼
┌──────────────────────────────────────────┐
│              Rally-go                       │
│                                          │
│  ┌──── SOCKS5 + Load Balancer ─────────┐ │
│  │   connection-level roundrobin/least │ │
│  └──────┬──────────┬────────┬─────────┘  │
│         │          │        │           │
│  ┌──────▼──┐ ┌────▼──┐ ┌───▼──────┐    │
│  │Hysteria2│ │  SS   │ │ Trojan   │    │
│  │ → VPS 1 │ │→ VPS 2│ │ → VPS 3 │    │
│  └─────────┘ └───────┘ └──────────┘    │
└──────────────────────────────────────────┘
```

## Quick Start

### 1. Configuration

```yaml
# rally.yaml
bind: ":1080"
balance: roundrobin

vps:
  - name: hk1
    type: hysteria2
    server: hk1.example.com
    port: 23872
    password: "your-password"
    down_mbps: 500

  - name: jp1
    type: ss
    server: jp1.example.com
    port: 8388
    password: "ss-password"
    cipher: AEAD_CHACHA20_POLY1305
```

> **⚠️ TLS Note**: Hysteria2 uses TLS for transport. If your VPS uses a self-signed certificate, or `sni` points to an IP instead of the certificate's domain, you **must** set `insecure: true`. Always set `sni` to the domain name on your VPS certificate.

### 2. Run

```bash
# Native
rally run -c rally.yaml

# With Web UI (default 127.0.0.1:9090)
rally run -c rally.yaml --web

# Docker (set RALLY_WEB_TOKEN when exposing the Web UI)
docker run -d -p 1080:1080 -p 9090:9090 \
  -e RALLY_WEB_TOKEN="change-me" \
  -v ./rally.yaml:/etc/rally.yaml \
  ghcr.io/zmenggg/rally-go:latest \
  run -c /etc/rally.yaml --web :9090
```

### 3. Use

```bash
curl -x socks5://127.0.0.1:1080 https://www.youtube.com/watch?v=...
```

Open http://localhost:9090 for the Web dashboard.

The Web UI binds to localhost by default. To listen on `:9090` or another
public address, set `RALLY_WEB_TOKEN` and authenticate with Basic Auth
(any username, token as password) or `Authorization: Bearer <token>`.

## Configuration Reference

```yaml
bind: ":1080"              # SOCKS5 listen address
balance: roundrobin        # Strategy: roundrobin | leastconn

log:
  level: info              # debug | info | warn | error
  output: ""               # log file path (empty = stderr)

vps:
  - name: my-node          # Node name
    type: hysteria2        # Protocol: hysteria2 / socks5 / ss / trojan / vless
    server: 1.2.3.4        # Server address
    port: 23872            # Server port
    password: "secret"     # Auth password
    sni: ""                # TLS SNI (defaults to server)
    enabled: true          # Enable node (default true)

    # Insecure TLS (Hysteria2 self-signed certs)
    insecure: false        # Skip TLS verification (default false)

    # Health check
    health_timeout: 15     # Health check timeout (seconds, default 15)

    # Shadowsocks specific
    cipher: AEAD_CHACHA20_POLY1305

    # VLESS specific
    uuid: "..."            # UUID
    flow: "xtls-rprx-vision"

    # Hysteria2 specific
    down_mbps: 500         # Downlink bandwidth (Mbps)
    up_mbps: 50            # Uplink bandwidth (Mbps)
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `rally run -c rally.yaml` | Start proxy |
| `rally run -c rally.yaml --web` | Start proxy + Web UI |
| `rally run -c rally.yaml --web :8888` | Custom Web UI port |
| `rally web -c rally.yaml` | Start Web UI only |
| `rally web -c rally.yaml --addr :8888` | Custom port |
| `rally check -c rally.yaml` | Validate config |
| `rally reload` | Send SIGHUP for hot reload |
| `rally version` | Print version |

## Web UI

| Page | Features |
|------|----------|
| **Dashboard** | Node status, live per-node & aggregate throughput, cumulative traffic, node toggle |
| **Nodes** | Visual add/edit/delete nodes |
| **Config** | YAML source editor |
| **Logs** | Real-time log streaming (SSE) |

Supports **English / 中文** language switching.

## How Bandwidth Aggregation Works

Rally-go does **connection-level aggregation**, not packet-level:

| Scenario | Works? | Explanation |
|----------|--------|-------------|
| Download 10 files simultaneously | ✅ **Yes** | Each connection goes to a different VPS |
| Browse a webpage (dozens of requests) | ✅ **Yes** | Requests distributed across VPSes |
| Single TCP stream (one large file) | ⚠️ **Single VPS** | One connection can only use one VPS |

**Best practice:** use multi-connection downloaders (MeTube 12 connections, qBittorrent,
curl --parallel, yt-dlp) for maximum aggregation.

## Comparison

| Feature | sing-box + HAProxy | Rally-go |
|---------|-------------------|-------|
| Components | 2 containers | 1 binary |
| License | GPLv3 | MIT |
| Config format | JSON + CFG | YAML |
| Protocols | Hysteria2 | Hysteria2 / SOCKS5 / SS / Trojan / VLESS |
| Hot reload | ❌ | ✅ SIGHUP |
| Management UI | ❌ | ✅ Web Dashboard (i18n) |
| Traffic stats | ❌ | ✅ Live rates + cumulative |
| Health check | ❌ | ✅ Auto health checks (30s interval) |
| Deploy | Docker only | Docker + native |

## Development

```bash
# Prerequisites: Go 1.24+
git clone https://github.com/ZMenggg/Rally-go.git
cd Rally-go
go build -o rally ./cmd/rally/
./rally run -c rally.yaml
```

## License

MIT
