# Architecture

## Overview

Rally is a **connection-level bandwidth aggregator** for multiple VPS proxy connections.

## How It Works

```
App -> SOCKS5 :1080 -> Rally TCP Forwarder (roundrobin)
                      |-- Hysteria2 Tunnel -> VPS 1
                      |-- Hysteria2 Tunnel -> VPS 2
                      '-- Hysteria2 Tunnel -> VPS 3
```

### Key Components

| Component | Role | Implementation |
|---|---|---|
| SOCKS5 Listener | Accepts app connections | Go `net.Listener` |
| TCP Forwarder | Relays TCP streams | Go stdlib `io.Copy` |
| Load Balancer | Distributes connections | Atomic round-robin counter |
| Hysteria2 Client | Encrypted tunnel to each VPS | `apernet/hysteria/core/v2` |

## Bandwidth Aggregation

Rally operates at the **TCP connection level**:

| Scenario | Behaviour |
|---|---|
| Multiple concurrent connections | Each one goes to a different VPS -> bandwidth adds up |
| Single connection | Stays on one VPS -> no aggregation for that stream |

## Protocol Support

### Current: Hysteria2
### Planned: Shadowsocks, VLESS+Reality, Trojan, SOCKS5 direct