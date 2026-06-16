# Welcome to Rally Wiki

**Rally** fuses multiple VPS proxy connections into a single SOCKS5 endpoint, aggregating their bandwidth for faster downloads.

## Quick Links

- [[Configuration]] - Detailed config reference
- [[Architecture]] - How Rally works
- [[CLI Reference]] - Command reference

## Getting Started

```yaml
bind: ":1080"
vps:
  - name: us1
    type: hysteria2
    server: your-server.com
    port: 23872
    password: "your-password"
```

```bash
docker run -d -p 1080:1080 -v ./rally.yaml:/etc/rally.yaml ghcr.io/zmenggg/rally:latest
```