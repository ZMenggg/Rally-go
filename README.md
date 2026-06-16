# Rally — Multi-VPS Bandwidth Aggregation Proxy

Rally fuses multiple VPS proxy connections into a single SOCKS5 endpoint,
aggregating their bandwidth for faster downloads.

## Architecture

```
App → SOCKS5 :1080 → Rally
                      ├─ Hysteria2 → VPS 1
                      ├─ Hysteria2 → VPS 2
                      └─ Hysteria2 → VPS 3
```

## Quick Start

```yaml
# rally.yaml
bind: ":1080"
balance: roundrobin
vps:
  - name: vps1
    type: hysteria2
    server: your-server.com
    port: 23872
    password: "your-password"
```

```bash
# Docker
docker run -d -p 1080:1080 -v ./rally.yaml:/etc/rally.yaml \
  ghcr.io/zmenggg/rally:latest

# Native (with Go installed)
go install github.com/ZMenggg/Rally/cmd/rally@latest
rally run -c rally.yaml
```

## License

MIT
