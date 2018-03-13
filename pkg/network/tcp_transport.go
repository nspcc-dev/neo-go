package network

import (
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

// TCPTransport allows network communication over TCP.
type TCPTransport struct {
	server   *Server
	listener net.Listener
	bindAddr string
	proto    chan protoTuple
}

// NewTCPTransport return a new TCPTransport that will listen for
// new incoming peer connections.
func NewTCPTransport(s *Server, bindAddr string) *TCPTransport {
	return &TCPTransport{
		server:   s,
		bindAddr: bindAddr,
		proto:    make(chan protoTuple),
	}
}

// Consumer implements the Transporter interface.
func (t *TCPTransport) Consumer() <-chan protoTuple {
	return t.proto
}

// Dial implements the Transporter interface.
func (t *TCPTransport) Dial(addr string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	go t.handleConn(conn)
	return nil
}

// Accept implements the Transporter interface.
func (t *TCPTransport) Accept() {
	l, err := net.Listen("tcp", t.bindAddr)
	if err != nil {
		log.Fatalf("TCP listen error %s", err)
		return
	}

	t.listener = l

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Warnf("TCP accept error: %s", err)
			continue
		}
		go t.handleConn(conn)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	p := NewTCPPeer(conn, t.proto)
	t.server.register <- p
	// This will block until the peer is stopped running.
	p.run(t.proto)
	log.Warnf("TCP released peer: %s", p.Endpoint())
}

// Close implements the Transporter interface.
func (t *TCPTransport) Close() {
	t.listener.Close()
}

// Proto implements the Transporter interface.
func (t *TCPTransport) Proto() string {
	return "tcp"
}
