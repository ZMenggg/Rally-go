# Rally-go ⚡

**Rally-go** — 将多台 VPS 代理连接融合为一条高速管道，聚合带宽加速下载。

单 Go 二进制，MIT 协议，Docker / 裸机均可部署。

中文 | [English](README.en.md)

---

## 特性

- **多协议支持** — Hysteria2、SOCKS5、Shadowsocks、Trojan、VLESS
- **带宽聚合** — 连接级 roundrobin / leastconn 负载均衡
- **Web 管理界面** — 可视化节点管理、实时流量监控、日志查看（中英文）
- **热重载** — `SIGHUP` 信号触发配置重载，无需重启进程
- **实时监控** — 每节点速率、总吞吐量、累计流量一目了然
- **节点开关** — 仪表盘一键启用/禁用节点
- **健康检查** — 后台自动检测节点连通性，异常节点自动移出轮询池

## 架构

```text
应用 (任何支持 SOCKS5 的软件)
    │  socks5://127.0.0.1:1080
    ▼
┌──────────────────────────────────────────┐
│              Rally-go                       │
│                                          │
│  ┌─── SOCKS5 处理 + 负载均衡 ─────────┐  │
│  │   连接级 roundrobin / leastconn     │  │
│  └──────┬──────────┬────────┬─────────┘  │
│         │          │        │           │
│  ┌──────▼──┐ ┌────▼──┐ ┌───▼──────┐    │
│  │Hysteria2│ │  SS   │ │ Trojan   │    │
│  │ → VPS 1 │ │→ VPS 2│ │ → VPS 3 │    │
│  └─────────┘ └───────┘ └──────────┘    │
└──────────────────────────────────────────┘
```

## 快速开始

### 1. 配置

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

> **⚠️ TLS 注意**：Hysteria2 使用 TLS 加密传输。如果 VPS 的证书是自签名、或 `sni` 指向 IP 而非证书域名，必须设置 `insecure: true`。建议始终将 `sni` 设为 VPS 证书的域名。

### 2. 运行

```bash
# 裸机
rally run -c rally.yaml

# 同时启动 Web 管理界面（默认 127.0.0.1:9090）
rally run -c rally.yaml --web

# Docker（公开 Web UI 时必须设置 RALLY_WEB_TOKEN）
docker run -d -p 1080:1080 -p 9090:9090 \
  -e RALLY_WEB_TOKEN="change-me" \
  -v ./rally.yaml:/etc/rally.yaml \
  ghcr.io/zmenggg/rally-go:latest \
  run -c /etc/rally.yaml --web :9090
```

### 3. 使用

```bash
curl -x socks5://127.0.0.1:1080 https://www.youtube.com/watch?v=...
```

打开 http://localhost:9090 查看 Web 管理界面。

Web UI 默认只监听本机地址。若需要监听 `:9090` 或其他公网地址，请设置
`RALLY_WEB_TOKEN`，访问时使用 Basic Auth（用户名任意，密码为 token）或
`Authorization: Bearer <token>`。

## 配置参考

```yaml
bind: ":1080"              # SOCKS5 监听地址
balance: roundrobin        # 负载均衡: roundrobin | leastconn

log:
  level: info              # debug | info | warn | error
  output: ""               # 日志文件路径(空=stderr)

vps:
  - name: my-node          # 节点名称
    type: hysteria2        # 协议: hysteria2 / socks5 / ss / trojan / vless
    server: 1.2.3.4        # 服务器地址
    port: 23872            # 服务器端口
    password: "secret"     # 认证密码
    sni: ""                # TLS SNI（默认用 server）
    enabled: true          # 是否启用（默认 true）

    # 不安全 TLS（Hysteria2 自签名证书场景）
    insecure: false        # 跳过 TLS 证书验证（默认 false）

    # 健康检查
    health_timeout: 15     # 健康检查超时秒数（默认 15）

    # Shadowsocks 专用
    cipher: AEAD_CHACHA20_POLY1305  # 加密方式

    # VLESS 专用
    uuid: "..."            # UUID
    flow: "xtls-rprx-vision"  # 流控

    # Hysteria2 专用
    down_mbps: 500         # 下行带宽
    up_mbps: 50            # 上行带宽
```

## CLI 命令

| 命令 | 说明 |
|------|------|
| `rally run -c rally.yaml` | 启动代理 |
| `rally run -c rally.yaml --web` | 启动代理 + Web UI |
| `rally run -c rally.yaml --web :8888` | 指定 Web UI 端口 |
| `rally web -c rally.yaml` | 仅启动 Web UI |
| `rally web -c rally.yaml --addr :8888` | 指定端口 |
| `rally check -c rally.yaml` | 校验配置 |
| `rally reload` | 发送 SIGHUP 热重载 |
| `rally version` | 版本信息 |

## Web UI

| 页面 | 功能 |
|------|------|
| **Dashboard** | 节点状态、每节点实时速率、汇总吞吐量、累计流量、节点开关 |
| **Nodes** | 可视化添加/编辑/删除节点 |
| **Config** | YAML 源码编辑器 |
| **Logs** | 实时日志查看（SSE 推送） |

支持 **中文 / English** 界面切换。

## 带宽聚合原理

Rally-go 做的是 **连接级聚合**，不是数据包级聚合：

| 场景 | 是否聚合 | 说明 |
|------|---------|------|
| 同时下载 10 个文件 | ✅ **是** | 每个文件连接走不同 VPS |
| 浏览网页（几十个请求） | ✅ **是** | 请求分发到各 VPS |
| 单一大文件单连接下载 | ⚠️ **仅一路** | 一条连接只能用一台 VPS |

**最佳实践：** 配合支持多路并发的下载工具（MeTube、qBittorrent、curl --parallel、yt-dlp），根据聚合的节点数量，手动调整下载器并发数量，可达到最优聚合效果。

## 与其他方案对比

| 特性 | sing-box + HAProxy | Rally-go |
|------|-------------------|-------|
| 组件数 | 2 容器 | 1 二进制 |
| 许可证 | GPLv3 | MIT |
| 配置格式 | JSON + CFG | YAML |
| 协议支持 | Hysteria2 | Hysteria2 / SOCKS5 / SS / Trojan / VLESS |
| 热加载 | ❌ | ✅ SIGHUP |
| 管理面板 | ❌ | ✅ Web UI（中英文） |
| 流量监控 | ❌ | ✅ 实时速率 + 累计流量 |
| 健康检查 | ❌ | ✅ 自动检测，30秒间隔 |
| 部署方式 | Docker 仅 | Docker + 裸机 |

## 开发

```bash
# 依赖: Go 1.24+
git clone https://github.com/ZMenggg/Rally-go.git
cd Rally-go
go build -o rally ./cmd/rally/
./rally run -c rally.yaml
```

## License

MIT
