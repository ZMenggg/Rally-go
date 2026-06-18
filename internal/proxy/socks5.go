package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/ZMenggg/Rally-go/internal/logger"
)

const (
	socksVer5             = 0x05
	socksCmdConnect       = 0x01
	socksAtypIPv4         = 0x01
	socksAtypDomain       = 0x03
	socksAtypIPv6         = 0x04
	socksRepSuccess       = 0x00
	socksRepFailure       = 0x01
	socksRepNotAllowed    = 0x02
	socksRepUnreachable   = 0x04
	socksHandshakeTimeout = 10 * time.Second
)

// SOCKS5Proxy handles a single SOCKS5 connection.
type SOCKS5Proxy struct {
	Dial func(addr string) (net.Conn, error)
}

// Handle processes a SOCKS5 handshake.
func (p *SOCKS5Proxy) Handle(client net.Conn) {
	defer client.Close()

	if err := client.SetDeadline(time.Now().Add(socksHandshakeTimeout)); err != nil {
		logger.Debug("socks5 set deadline: %v", err)
	}
	targetAddr, err := p.negotiate(client)
	if err != nil {
		logger.Debug("socks5 negotiate: %v", err)
		return
	}
	if err := client.SetDeadline(time.Time{}); err != nil {
		logger.Debug("socks5 clear deadline: %v", err)
	}

	remote, err := p.Dial(targetAddr)
	if err != nil {
		logger.Debug("socks5 dial %s: %v", targetAddr, err)
		p.sendReply(client, socksRepUnreachable, nil)
		return
	}
	defer remote.Close()

	p.sendReply(client, socksRepSuccess, nil)

	// Bidirectional copy (stats tracked via statsConn wrapper)
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(remote, client)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(client, remote)
		done <- struct{}{}
	}()
	<-done
	<-done
}

// negotiate performs SOCKS5 handshake and returns target address.
func (p *SOCKS5Proxy) negotiate(conn net.Conn) (string, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", fmt.Errorf("read greeting: %w", err)
	}
	if header[0] != socksVer5 {
		return "", fmt.Errorf("unsupported SOCKS version: %d", header[0])
	}
	nMethods := int(header[1])
	if nMethods < 1 {
		return "", fmt.Errorf("no methods advertised")
	}
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", fmt.Errorf("read methods: %w", err)
	}

	if _, err := conn.Write([]byte{socksVer5, 0x00}); err != nil {
		return "", fmt.Errorf("write method reply: %w", err)
	}

	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return "", fmt.Errorf("read request header: %w", err)
	}
	if reqHeader[0] != socksVer5 {
		return "", fmt.Errorf("unsupported request version: %d", reqHeader[0])
	}
	if reqHeader[1] != socksCmdConnect {
		return "", fmt.Errorf("unsupported command: %d (only CONNECT supported)", reqHeader[1])
	}

	atyp := reqHeader[3]
	var host string
	switch atyp {
	case socksAtypIPv4:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", fmt.Errorf("read IPv4: %w", err)
		}
		host = net.IP(addr).String()
	case socksAtypDomain:
		lenByte := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenByte); err != nil {
			return "", fmt.Errorf("read domain length: %w", err)
		}
		domain := make([]byte, int(lenByte[0]))
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", fmt.Errorf("read domain: %w", err)
		}
		host = string(domain)
	case socksAtypIPv6:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", fmt.Errorf("read IPv6: %w", err)
		}
		host = net.IP(addr).String()
	default:
		return "", fmt.Errorf("unsupported address type: %d", atyp)
	}

	port := make([]byte, 2)
	if _, err := io.ReadFull(conn, port); err != nil {
		return "", fmt.Errorf("read port: %w", err)
	}

	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port)))), nil
}

func (p *SOCKS5Proxy) sendReply(conn net.Conn, rep byte, bindAddr net.Addr) {
	var atyp byte = socksAtypIPv4
	addrBytes := net.IP{0, 0, 0, 0}
	portBytes := make([]byte, 2)

	if bindAddr != nil {
		switch a := bindAddr.(type) {
		case *net.TCPAddr:
			if ip4 := a.IP.To4(); ip4 != nil {
				atyp = socksAtypIPv4
				addrBytes = ip4
			} else {
				atyp = socksAtypIPv6
				addrBytes = a.IP.To16()
			}
			binary.BigEndian.PutUint16(portBytes, uint16(a.Port))
		}
	}

	resp := append([]byte{socksVer5, rep, 0x00, atyp}, addrBytes...)
	resp = append(resp, portBytes...)
	conn.Write(resp)
}
