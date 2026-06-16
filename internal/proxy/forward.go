package proxy

import (
	"io"
	"log"
	"net"
)

// Forwarder handles proxying TCP connections from a local listener
// to backend SOCKS5 proxies in a round-robin fashion.
type Forwarder struct {
	next func() string // returns backend address
}

// NewForwarder creates a Forwarder that picks backends via the given function.
func NewForwarder(next func() string) *Forwarder {
	return &Forwarder{next: next}
}

// Serve starts a TCP listener and forwards connections.
func (f *Forwarder) Serve(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("SOCKS5 proxy listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go f.handleConn(conn)
	}
}

func (f *Forwarder) handleConn(client net.Conn) {
	defer client.Close()

	backendAddr := f.next()
	if backendAddr == "" {
		log.Printf("no backends available")
		return
	}

	backend, err := net.Dial("tcp", backendAddr)
	if err != nil {
		log.Printf("dial backend %s: %v", backendAddr, err)
		return
	}
	defer backend.Close()

	// Bidirectional copy
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(backend, client)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(client, backend)
		done <- struct{}{}
	}()
	<-done
}
