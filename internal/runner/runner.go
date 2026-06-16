package runner

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/ZMenggg/Rally/internal/balancer"
	"github.com/ZMenggg/Rally/internal/config"
	"github.com/ZMenggg/Rally/internal/proxy"
)

// Runner manages the lifecycle of the proxy server.
type Runner struct {
	cfg *config.Config
}

// New creates a new Runner.
func New(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg}
}

// Run starts the proxy server.
func (r *Runner) Run() error {
	backends := r.buildBackends()
	if len(backends) == 0 {
		return fmt.Errorf("no VPS backends configured")
	}

	b := balancer.New(backends)

	// Start Hysteria2 client for each VPS
	for _, vps := range r.cfg.VPS {
		if err := r.startClient(vps); err != nil {
			log.Printf("warn: failed to start client %s: %v", vps.Name, err)
		}
	}

	// Start the forwarder
	next := func() string {
		be := b.Next()
		if be == nil {
			return ""
		}
		b.Acquire(be)
		return be.Addr
	}

	fwd := proxy.NewForwarder(next)
	return fwd.Serve(r.cfg.Bind)
}

func (r *Runner) buildBackends() []*balancer.Backend {
	backends := make([]*balancer.Backend, 0, len(r.cfg.VPS))
	for i, vps := range r.cfg.VPS {
		port := 11080 + i
		backends = append(backends, &balancer.Backend{
			Name:   vps.Name,
			Addr:   net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
			Weight: 1,
		})
	}
	return backends
}

func (r *Runner) startClient(vps config.VPS) error {
	switch vps.Type {
	case "hysteria2", "":
		return r.startHysteria2(vps)
	default:
		return fmt.Errorf("unsupported protocol: %s", vps.Type)
	}
}

func (r *Runner) startHysteria2(vps config.VPS) error {
	log.Printf("starting Hysteria2 client for %s (%s:%d)", vps.Name, vps.Server, vps.Port)
	// TODO: Implement Hysteria2 client integration
	// This will use github.com/apernet/hysteria/core/v2/client
	// For now, return a placeholder
	log.Printf("[TODO] Hysteria2 client for %s will listen on %s", vps.Name, "127.0.0.1:11080")
	return nil
}
