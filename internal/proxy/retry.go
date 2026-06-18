package proxy

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/ZMenggg/Rally-go/internal/logger"
)

// RetryProvider wraps a ConnProvider and automatically reconnects on Dial failure.
// It uses a factory function to create a new provider when reconnection is needed.
type RetryProvider struct {
	name     string
	factory  func() (ConnProvider, error)
	provider atomic.Value
	mu       sync.Mutex
}

type closeableProvider interface {
	Close() error
}

// NewRetryProvider creates a RetryProvider.
func NewRetryProvider(name string, initial ConnProvider, factory func() (ConnProvider, error)) *RetryProvider {
	p := &RetryProvider{
		name:    name,
		factory: factory,
	}
	if initial != nil {
		p.provider.Store(initial)
	}
	return p
}

func (p *RetryProvider) Name() string { return p.name }

func (p *RetryProvider) Dial(addr string) (net.Conn, error) {
	provider := p.provider.Load()
	if provider == nil {
		// No initial connection — attempt reconnect immediately
		return p.reconnect(addr)
	}
	conn, err := provider.(ConnProvider).Dial(addr)
	if err == nil {
		return conn, nil
	}

	// Dial failed — attempt to reconnect
	logger.Warn("%s: dial error (%v), reconnecting...", p.name, err)
	return p.reconnect(addr)
}

func (p *RetryProvider) reconnect(addr string) (net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 0; i < 3; i++ {
		newProvider, err := p.factory()
		if err != nil {
			logger.Warn("%s: reconnect attempt %d failed: %v", p.name, i+1, err)
			continue
		}
		old := p.provider.Load()
		p.provider.Store(newProvider)
		if closer, ok := old.(closeableProvider); ok {
			_ = closer.Close()
		}
		logger.Info("%s: reconnected on attempt %d", p.name, i+1)
		return newProvider.Dial(addr)
	}
	return nil, fmt.Errorf("%s: all 3 reconnect attempts failed", p.name)
}
