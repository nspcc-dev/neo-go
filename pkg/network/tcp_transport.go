package network

import (
	"net"
	"os"
	"time"

	log "github.com/go-kit/kit/log"
)

// TCPTransport allows network communication over TCP.
type TCPTransport struct {
	server   *Server
	listener net.Listener
	logger   log.Logger
	bindAddr string
	proto    chan protoTuple
}

// NewTCPTransport return a new TCPTransport.
func NewTCPTransport(bindAddr string) *TCPTransport {
	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	l = log.With(l, "TCP")
	return &TCPTransport{
		logger:   l,
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
func (t *TCPTransport) Accept(s *Server) {
	l, err := net.Listen("tcp", t.bindAddr)
	if err != nil {
		t.logger.Log("msg", "listen error", "err", err)
		return
	}

	t.server = s
	t.listener = l

	for {
		conn, err := l.Accept()
		if err != nil {
			t.logger.Log("msg", "accept error", "err", err)
			continue
		}
		go t.handleConn(conn)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	p := NewTCPPeer(conn, t.proto)
	t.server.register <- p
	if err := p.run(t.proto); err != nil {
		t.server.unregister <- peerDrop{
			peer:   p,
			reason: err,
		}
	}
}

// Close implements the Transporter interface.
func (t *TCPTransport) Close() {
	t.listener.Close()
}

// Proto implements the Transporter interface.
func (t *TCPTransport) Proto() string {
	return "tcp"
}
