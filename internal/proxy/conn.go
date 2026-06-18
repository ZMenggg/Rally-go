package proxy

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	sscore "github.com/shadowsocks/go-shadowsocks2/core"
)

// ─── SOCKS5 ConnProvider ────────────────────────────────────────────────────

// SOCKS5Provider connects to a remote SOCKS5 proxy and performs the
// SOCKS5 handshake to request a connection to the target address.
type SOCKS5Provider struct {
	name   string
	server string
}

// NewSOCKS5Provider creates a new SOCKS5Provider.
func NewSOCKS5Provider(name, server string) *SOCKS5Provider {
	return &SOCKS5Provider{name: name, server: server}
}

func (p *SOCKS5Provider) Name() string { return p.name }

func (p *SOCKS5Provider) Dial(addr string) (net.Conn, error) {
	// Connect to the remote SOCKS5 proxy
	proxyConn, err := net.DialTimeout("tcp", p.server, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to SOCKS5 proxy: %w", err)
	}

	// Perform SOCKS5 handshake as a client
	if err := socks5ClientHandshake(proxyConn, addr); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("SOCKS5 handshake: %w", err)
	}

	return proxyConn, nil
}

// socks5ClientHandshake performs a SOCKS5 client handshake on an established
// connection, requesting a CONNECT to targetAddr.
func socks5ClientHandshake(conn net.Conn, targetAddr string) error {
	// Method negotiation: send ver=5, nMethods=1, method=0x00 (no auth)
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("write greeting: %w", err)
	}

	// Read response: ver=5, method
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("read method response: %w", err)
	}
	if resp[0] != 0x05 {
		return fmt.Errorf("unexpected SOCKS version: %d", resp[0])
	}
	if resp[1] != 0x00 {
		return fmt.Errorf("SOCKS5 auth method not accepted: %d", resp[1])
	}

	// Build CONNECT request
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return fmt.Errorf("split target address: %w", err)
	}
	port, err := parsePort(portStr)
	if err != nil {
		return fmt.Errorf("parse target port: %w", err)
	}

	var req []byte
	req = append(req, 0x05, 0x01, 0x00) // ver, cmd=CONNECT, rsv

	ip := net.ParseIP(host)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req = append(req, 0x01) // IPv4
			req = append(req, ip4...)
		} else {
			req = append(req, 0x04) // IPv6
			req = append(req, ip.To16()...)
		}
	} else {
		// Domain name
		if len(host) > 255 {
			return fmt.Errorf("domain name too long: %d", len(host))
		}
		req = append(req, 0x03) // DOMAINNAME
		req = append(req, byte(len(host)))
		req = append(req, []byte(host)...)
	}

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	req = append(req, portBytes...)

	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("write connect request: %w", err)
	}

	// Read response: ver, rep, rsv, atyp, bind.addr, bind.port
	respHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, respHeader); err != nil {
		return fmt.Errorf("read connect response: %w", err)
	}
	if respHeader[0] != 0x05 {
		return fmt.Errorf("unexpected SOCKS version in response: %d", respHeader[0])
	}
	if respHeader[1] != 0x00 {
		return fmt.Errorf("SOCKS5 request rejected: code %d", respHeader[1])
	}

	// Read the bind address (we don't need it)
	atyp := respHeader[3]
	switch atyp {
	case 0x01: // IPv4
		_, err = io.ReadFull(conn, make([]byte, 4+2))
	case 0x03: // DOMAINNAME
		lenByte := make([]byte, 1)
		if _, err = io.ReadFull(conn, lenByte); err == nil {
			_, err = io.ReadFull(conn, make([]byte, int(lenByte[0])+2))
		}
	case 0x04: // IPv6
		_, err = io.ReadFull(conn, make([]byte, 16+2))
	}
	return err
}

// ─── Shadowsocks ConnProvider ───────────────────────────────────────────────

// ShadowsocksProvider connects to a remote Shadowsocks server.
type ShadowsocksProvider struct {
	name   string
	server string
	cipher sscore.Cipher
}

// NewShadowsocksProvider creates a new ShadowsocksProvider.
func NewShadowsocksProvider(name, server, cipherName, password string) (*ShadowsocksProvider, error) {
	if cipherName == "" {
		cipherName = "AEAD_CHACHA20_POLY1305"
	}
	cipher, err := sscore.PickCipher(cipherName, nil, password)
	if err != nil {
		return nil, fmt.Errorf("pick cipher: %w", err)
	}
	return &ShadowsocksProvider{
		name:   name,
		server: server,
		cipher: cipher,
	}, nil
}

func (p *ShadowsocksProvider) Name() string { return p.name }

func (p *ShadowsocksProvider) Dial(addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", p.server, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to shadowsocks server: %w", err)
	}

	// Wrap with encryption
	shadowed := p.cipher.StreamConn(conn)

	// Send target address (SOCKS5 addr format: [atype][addr][port])
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("split target address: %w", err)
	}
	port, err := parsePort(portStr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("parse target port: %w", err)
	}

	var header []byte
	ip := net.ParseIP(host)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			header = append(header, 0x01) // IPv4
			header = append(header, ip4...)
		} else {
			header = append(header, 0x04) // IPv6
			header = append(header, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			conn.Close()
			return nil, fmt.Errorf("domain too long: %d", len(host))
		}
		header = append(header, 0x03) // DOMAINNAME
		header = append(header, byte(len(host)))
		header = append(header, []byte(host)...)
	}

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	header = append(header, portBytes...)

	if _, err := shadowed.Write(header); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send target address: %w", err)
	}

	return &ssConn{raw: conn, wrapped: shadowed}, nil
}

// ssConn ensures both the raw connection and the shadowed connection get closed.
type ssConn struct {
	raw     net.Conn
	wrapped net.Conn
}

func (c *ssConn) Read(b []byte) (int, error)         { return c.wrapped.Read(b) }
func (c *ssConn) Write(b []byte) (int, error)        { return c.wrapped.Write(b) }
func (c *ssConn) Close() error                       { return c.raw.Close() }
func (c *ssConn) LocalAddr() net.Addr                { return c.raw.LocalAddr() }
func (c *ssConn) RemoteAddr() net.Addr               { return c.raw.RemoteAddr() }
func (c *ssConn) SetDeadline(t time.Time) error      { return c.raw.SetDeadline(t) }
func (c *ssConn) SetReadDeadline(t time.Time) error  { return c.raw.SetReadDeadline(t) }
func (c *ssConn) SetWriteDeadline(t time.Time) error { return c.raw.SetWriteDeadline(t) }

// ─── Trojan ConnProvider ────────────────────────────────────────────────────

// TrojanProvider connects to a remote Trojan server.
// Trojan protocol: TLS connection, then send `password\r\n` + request.
type TrojanProvider struct {
	name     string
	server   string
	password string
	sni      string
}

// NewTrojanProvider creates a new TrojanProvider.
func NewTrojanProvider(name, server, password, sni string) *TrojanProvider {
	if sni == "" {
		sni = server
	}
	return &TrojanProvider{
		name:     name,
		server:   server,
		password: password,
		sni:      sni,
	}
}

func (p *TrojanProvider) Name() string { return p.name }

func (p *TrojanProvider) Dial(addr string) (net.Conn, error) {
	// 1. TLS handshake
	tlsConn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second},
		"tcp", p.server, &tls.Config{
			ServerName: p.sni,
		})
	if err != nil {
		return nil, fmt.Errorf("trojan TLS dial: %w", err)
	}

	// 2. Send password + CRLF
	payload := []byte(p.password + "\r\n")

	// 3. Trojan request format:
	//    [atyp (1)] [addr (var)] [port (2)] [CRLF]
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("split target address: %w", err)
	}
	port, err := parsePort(portStr)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("parse target port: %w", err)
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			payload = append(payload, 0x01) // IPv4
			payload = append(payload, ip4...)
		} else {
			payload = append(payload, 0x04) // IPv6
			payload = append(payload, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			tlsConn.Close()
			return nil, fmt.Errorf("domain too long: %d", len(host))
		}
		payload = append(payload, 0x03) // DOMAINNAME
		payload = append(payload, byte(len(host)))
		payload = append(payload, []byte(host)...)
	}

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	payload = append(payload, portBytes...)
	payload = append(payload, '\r', '\n')

	if _, err := tlsConn.Write(payload); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("trojan send request: %w", err)
	}

	return tlsConn, nil
}

// ─── VLESS ConnProvider ─────────────────────────────────────────────────────

// VLESSProvider connects to a remote VLESS server.
// VLESS protocol v1.0 (basic TCP proxy):
//
//	Request: 16-byte UUID + 1-byte version(0) + 1-byte cmd(1=TCP) + [atyp][addr][port]
//	Response: 1-byte version + 1-byte status
type VLESSProvider struct {
	name   string
	server string
	uuid   [16]byte
	flow   string
	sni    string
}

// NewVLESSProvider creates a new VLESSProvider.
func NewVLESSProvider(name, server, uuid, flow, sni string) (*VLESSProvider, error) {
	if sni == "" {
		sni = server
	}
	uid, err := parseUUID(uuid)
	if err != nil {
		return nil, fmt.Errorf("parse VLESS UUID: %w", err)
	}
	return &VLESSProvider{
		name:   name,
		server: server,
		uuid:   uid,
		flow:   flow,
		sni:    sni,
	}, nil
}

func (p *VLESSProvider) Name() string { return p.name }

func (p *VLESSProvider) Dial(addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", p.server, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to VLESS server: %w", err)
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("split target address: %w", err)
	}
	port, err := parsePort(portStr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("parse target port: %w", err)
	}

	// Build VLESS request:
	// [16-byte UUID] [ver=0] [cmd=1 TCP] [portBE] [atyp] [addr...]
	var req []byte
	req = append(req, p.uuid[:]...)
	req = append(req, 0x00) // version
	req = append(req, 0x01) // command: TCP

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	req = append(req, portBytes...)

	ip := net.ParseIP(host)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req = append(req, 0x01) // IPv4
			req = append(req, ip4...)
		} else {
			req = append(req, 0x04) // IPv6
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			conn.Close()
			return nil, fmt.Errorf("domain too long: %d", len(host))
		}
		req = append(req, 0x03) // DOMAINNAME
		req = append(req, byte(len(host)))
		req = append(req, []byte(host)...)
	}

	// Add flow control padding for xtls-rprx-vision
	if p.flow != "" {
		req = append(req, 0x00) // padding length 0
	}
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send VLESS request: %w", err)
	}

	// Read VLESS response: version(1) + status(1) + ...
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read VLESS response: %w", err)
	}
	// resp[0] = version (0), resp[1] = status (0 = success)
	if resp[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("VLESS request rejected: status %d", resp[1])
	}

	return conn, nil
}

// parseUUID parses a UUID string like "550e8400-e29b-41d4-a716-446655440000"
// into a 16-byte array.
func parseUUID(s string) ([16]byte, error) {
	var uuid [16]byte
	hex := s
	if len(s) == 36 {
		if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
			return uuid, fmt.Errorf("invalid UUID format")
		}
		hex = strings.ReplaceAll(s, "-", "")
	}
	if len(hex) != 32 {
		return uuid, fmt.Errorf("invalid UUID length: %d", len(s))
	}
	for i := 0; i < 32; i += 2 {
		hi, ok := unhex(rune(hex[i]))
		if !ok {
			return uuid, fmt.Errorf("invalid UUID hex at position %d", i)
		}
		lo, ok := unhex(rune(hex[i+1]))
		if !ok {
			return uuid, fmt.Errorf("invalid UUID hex at position %d", i+1)
		}
		uuid[i/2] = hi<<4 | lo
	}
	return uuid, nil
}

func parsePort(portStr string) (uint16, error) {
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return 0, err
	}
	return uint16(port), nil
}

func unhex(c rune) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return byte(c - '0'), true
	case 'a' <= c && c <= 'f':
		return byte(c - 'a' + 10), true
	case 'A' <= c && c <= 'F':
		return byte(c - 'A' + 10), true
	}
	return 0, false
}
