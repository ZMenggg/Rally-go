package runner

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	hyclient "github.com/apernet/hysteria/core/v2/client"

	"github.com/ZMenggg/Rally-go/internal/balancer"
	"github.com/ZMenggg/Rally-go/internal/config"
	"github.com/ZMenggg/Rally-go/internal/logger"
	"github.com/ZMenggg/Rally-go/internal/proxy"
)

// Runner manages the lifecycle of the proxy server.
type Runner struct {
	cfg     *config.Config
	clients []hyclient.Client
	cleanup []func()

	mu       sync.Mutex
	running  bool
	done     chan struct{} // closed when Run() fully exits
	cancel   chan struct{}
	balancer *balancer.Balancer
	fwd      *proxy.Forwarder
}

// New creates a new Runner.
func New(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg}
}

// Run starts the proxy server.
func (r *Runner) Run() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("already running")
	}
	r.running = true
	r.done = make(chan struct{})
	r.cancel = make(chan struct{})
	r.mu.Unlock()

	backends, err := r.buildBackends()
	if err != nil {
		return fmt.Errorf("build backends: %w", err)
	}
	if len(backends) == 0 {
		return fmt.Errorf("no VPS backends configured")
	}

	b := balancer.NewWithStrategy(backends, r.cfg.Balance)

	picker := func() (*proxy.Backend, func()) {
		be := b.Next()
		if be == nil {
			return nil, nil
		}
		b.Acquire(be)
		return &proxy.Backend{
			Name:         be.Name,
			ConnProvider: be.Provider,
		}, func() { b.Release(be) }
	}

	fwd := proxy.NewForwarder(picker)

	r.mu.Lock()
	r.balancer = b
	r.fwd = fwd
	r.mu.Unlock()

	logger.Info("Rally started with %d VPS backend(s), listening on %s",
		len(backends), r.cfg.Bind)

	// Start health checker (periodically tests each backend)
	hc := newHealthChecker(b.Backends(), b, 30*time.Second, 2)
	hc.Start()
	r.cleanup = append(r.cleanup, func() { hc.Stop() })

	err = fwd.Serve(r.cfg.Bind)

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()
	close(r.done)
	return err
}

// ReloadConfig stops the current proxy and restarts with a new config.
func (r *Runner) ReloadConfig(cfg *config.Config) error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return fmt.Errorf("not running")
	}
	if r.fwd != nil {
		r.fwd.Stop()
	}
	r.mu.Unlock()

	r.Close()

	r.mu.Lock()
	r.cfg = cfg
	done := r.done
	r.mu.Unlock()

	// Wait for previous Run() to fully exit
	<-done

	logger.Info("Config reloaded, restarting with %d VPS backend(s)", len(cfg.VPS))
	// Run in background so SIGHUP handler is not blocked
	go func() {
		if err := r.Run(); err != nil {
			logger.Error("Reload restart: %v", err)
		}
	}()

	return nil
}

// Close shuts down all tunnel clients.
func (r *Runner) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.clients {
		if c != nil {
			c.Close()
		}
	}
	r.clients = nil

	for _, fn := range r.cleanup {
		fn()
	}
	r.cleanup = nil
}

// Balancer returns the current balancer (may be nil before first Run).
// Forwarder returns the current forwarder (may be nil before first Run).
func (r *Runner) Forwarder() *proxy.Forwarder {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fwd
}

func (r *Runner) Balancer() *balancer.Balancer {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.balancer
}

// IsEnabled checks if a VPS should be active.
func isEnabled(vps config.VPS) bool {
	if vps.Enabled == nil {
		return true // default enabled
	}
	return *vps.Enabled
}

// buildBackends creates a Balancer backend for each VPS.
func (r *Runner) buildBackends() ([]*balancer.Backend, error) {
	var backends []*balancer.Backend

	for _, vps := range r.cfg.VPS {
		if !isEnabled(vps) {
			logger.Debug("skipping disabled node: %s", vps.Name)
			continue
		}
		provider, err := r.startClient(vps)
		if err != nil {
			logger.Warn("failed to start client for %s: %v", vps.Name, err)
			// Still add a failed provider — RetryProvider will attempt reconnection
			provider = nil
		}

		// Wrap with auto-reconnect wrapper
		vpsCopy := vps // capture for closure
		reconnectFactory := func() (proxy.ConnProvider, error) {
			return r.reconnectClient(vpsCopy)
		}
		rp := proxy.NewRetryProvider(vps.Name, provider, reconnectFactory)
		if provider == nil {
			// Initial connection failed — schedule immediate retry
			go func() {
				logger.Info("%s: initial connection failed, retrying...", rp.Name())
				rp.Dial("8.8.8.8:53") // trigger reconnect in background
			}()
		}

		backends = append(backends, &balancer.Backend{
			Name:     vps.Name,
			Provider: rp,
			Weight:   1,
		})
	}

	return backends, nil
}

// startClient starts the appropriate tunnel client for the given VPS config.
func (r *Runner) startClient(vps config.VPS) (proxy.ConnProvider, error) {
	switch vps.Type {
	case "hysteria2", "":
		return r.startHysteria2(vps)
	case "socks5":
		return r.startSocks5(vps), nil
	case "ss", "shadowsocks":
		return r.startShadowsocks(vps)
	case "trojan":
		return r.startTrojan(vps), nil
	case "vless":
		return r.startVLESS(vps)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", vps.Type)
	}
}

// reconnectClient creates a new client for reconnection without appending
// to the Runner's cleanup lists (the initial client handles that).
func (r *Runner) reconnectClient(vps config.VPS) (proxy.ConnProvider, error) {
	// Re-use startClient logic but the old client remains in r.clients
	// and will be cleaned up on Runner.Close().
	return r.startClient(vps)
}

// ─── Hysteria2 ──────────────────────────────────────────────────────────────

func (r *Runner) startHysteria2(vps config.VPS) (proxy.ConnProvider, error) {
	logger.Info("starting Hysteria2 client for %s (%s:%d)", vps.Name, vps.Server, vps.Port)

	serverAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(vps.Server, strconv.Itoa(vps.Port)))
	if err != nil {
		return nil, fmt.Errorf("resolve server addr: %w", err)
	}

	tlsServerName := vps.SNI
	if tlsServerName == "" {
		tlsServerName = vps.Server
	}

	cfg := &hyclient.Config{
		ServerAddr: serverAddr,
		Auth:       vps.Password,
		TLSConfig: hyclient.TLSConfig{
			ServerName:         tlsServerName,
			InsecureSkipVerify: vps.Insecure,
		},
		BandwidthConfig: hyclient.BandwidthConfig{
			MaxTx: uint64(vps.UpMbps) * 1_000_000 / 8,
			MaxRx: uint64(vps.DownMbps) * 1_000_000 / 8,
		},
	}

	client, _, err := hyclient.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("hysteria2 new client: %w", err)
	}

	r.mu.Lock()
	r.clients = append(r.clients, client)
	r.mu.Unlock()

	logger.Info("Hysteria2 client for %s connected", vps.Name)
	return &hyProvider{name: vps.Name, client: client}, nil
}

type hyProvider struct {
	name   string
	client hyclient.Client
}

func (p *hyProvider) Name() string { return p.name }
func (p *hyProvider) Dial(addr string) (net.Conn, error) {
	return p.client.TCP(addr)
}

// ─── SOCKS5 ─────────────────────────────────────────────────────────────────

func (r *Runner) startSocks5(vps config.VPS) proxy.ConnProvider {
	server := net.JoinHostPort(vps.Server, strconv.Itoa(vps.Port))
	logger.Info("starting SOCKS5 client for %s (%s)", vps.Name, server)
	return proxy.NewSOCKS5Provider(vps.Name, server)
}

// ─── Shadowsocks ────────────────────────────────────────────────────────────

func (r *Runner) startShadowsocks(vps config.VPS) (proxy.ConnProvider, error) {
	server := net.JoinHostPort(vps.Server, strconv.Itoa(vps.Port))
	logger.Info("starting Shadowsocks client for %s (%s cipher=%s)",
		vps.Name, server, vps.Cipher)
	return proxy.NewShadowsocksProvider(vps.Name, server, vps.Cipher, vps.Password)
}

// ─── Trojan ─────────────────────────────────────────────────────────────────

func (r *Runner) startTrojan(vps config.VPS) proxy.ConnProvider {
	server := net.JoinHostPort(vps.Server, strconv.Itoa(vps.Port))
	logger.Info("starting Trojan client for %s (%s)", vps.Name, server)
	return proxy.NewTrojanProvider(vps.Name, server, vps.Password, vps.SNI)
}

// ─── VLESS ──────────────────────────────────────────────────────────────────

func (r *Runner) startVLESS(vps config.VPS) (proxy.ConnProvider, error) {
	server := net.JoinHostPort(vps.Server, strconv.Itoa(vps.Port))
	logger.Info("starting VLESS client for %s (%s)", vps.Name, server)
	return proxy.NewVLESSProvider(vps.Name, server, vps.UUID, vps.Flow, vps.SNI)
}

// ─── Health Checker ──────────────────────────────────────────────────────────

// healthChecker periodically tests each backend's connectivity and marks
// unhealthy nodes so the balancer skips them.
type healthChecker struct {
	backends    []*balancer.Backend
	balancer    *balancer.Balancer
	interval    time.Duration
	maxFails    int
	target      string
	consecutive map[string]int
	mu          sync.Mutex
	stopCh      chan struct{}
	started     bool
}

func newHealthChecker(backends []*balancer.Backend, b *balancer.Balancer, interval time.Duration, maxFails int) *healthChecker {
	return &healthChecker{
		backends:    backends,
		balancer:    b,
		interval:    interval,
		maxFails:    maxFails,
		target:      "www.gstatic.com:443",
		consecutive: make(map[string]int),
		stopCh:      make(chan struct{}),
	}
}

func (hc *healthChecker) Start() {
	if hc.started {
		return
	}
	hc.started = true
	go func() {
		// Delay first check to let connections stabilise
		time.Sleep(5 * time.Second)
		hc.checkAll()

		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				hc.checkAll()
			case <-hc.stopCh:
				return
			}
		}
	}()
	logger.Info("Health checker started (interval=%s, maxFails=%d, target=%s)",
		hc.interval, hc.maxFails, hc.target)
}

func (hc *healthChecker) Stop() {
	if hc.stopCh != nil {
		close(hc.stopCh)
	}
}

func (hc *healthChecker) checkAll() {
	for _, be := range hc.backends {
		if be.Provider == nil {
			continue
		}
		healthy := hc.checkBackend(be)
		hc.updateBackend(be.Name, healthy)
	}
}

func (hc *healthChecker) checkBackend(be *balancer.Backend) bool {
	conn, err := be.Provider.Dial(hc.target)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (hc *healthChecker) updateBackend(name string, healthy bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if healthy {
		wasUnhealthy := hc.consecutive[name] >= hc.maxFails
		hc.consecutive[name] = 0
		if wasUnhealthy {
			hc.balancer.SetHealth(name, true)
			logger.Info("Health check: %s recovered, marked HEALTHY", name)
		}
	} else {
		hc.consecutive[name]++
		if hc.consecutive[name] == hc.maxFails {
			hc.balancer.SetHealth(name, false)
			logger.Warn("Health check: %s marked UNHEALTHY (%d consecutive failures)", name, hc.consecutive[name])
		} else if hc.consecutive[name] < hc.maxFails {
			logger.Debug("Health check: %s failed (%d/%d)", name, hc.consecutive[name], hc.maxFails)
		}
	}
}
