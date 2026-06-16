package balancer

import (
	"sync/atomic"
)

// Backend is an abstract proxy backend.
type Backend struct {
	Name   string
	Addr   string // SOCKS5 address of this backend (e.g., "127.0.0.1:1081")
	Weight int

	active int64 // atomic: current active connections
}

// BackendInfo is returned for status reporting.
type BackendInfo struct {
	Name      string `json:"name"`
	Addr      string `json:"addr"`
	Active    int64  `json:"active"`
	Connected bool   `json:"connected"`
}

// Balancer distributes connections across backends.
type Balancer struct {
	backends []*Backend
	counter  atomic.Uint64
}

// New creates a Balancer with the given backends.
func New(backends []*Backend) *Balancer {
	return &Balancer{
		backends: backends,
	}
}

// Next picks the next backend based on the balancing strategy.
func (b *Balancer) Next() *Backend {
	if len(b.backends) == 0 {
		return nil
	}
	// Round-robin
	n := b.counter.Add(1) - 1
	idx := int(n) % len(b.backends)
	return b.backends[idx]
}

// Acquire marks a connection as active on a backend.
func (b *Balancer) Acquire(be *Backend) {
	atomic.AddInt64(&be.active, 1)
}

// Release marks a connection as released from a backend.
func (b *Balancer) Release(be *Backend) {
	atomic.AddInt64(&be.active, -1)
}

// Backends returns the current backend list.
func (b *Balancer) Backends() []*Backend {
	return b.backends
}

// Info returns a snapshot of all backends.
func (b *Balancer) Info() []BackendInfo {
	info := make([]BackendInfo, len(b.backends))
	for i, be := range b.backends {
		info[i] = BackendInfo{
			Name:      be.Name,
			Addr:      be.Addr,
			Active:    atomic.LoadInt64(&be.active),
			Connected: true,
		}
	}
	return info
}
