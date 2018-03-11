package p2p

import (
	"net"
	"os"

	log "github.com/go-kit/kit/log"
)

// TCPTransport allows network communication over TCP.
type TCPTransport struct {
	listener net.Listener
	logger   log.Logger
	bindAddr string
}

func NewTCPTransport(bindAddr string) *TCPTransport {
	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	l = log.With(l, "TCP")
	return &TCPTransport{
		logger:   l,
		bindAddr: bindAddr,
	}
}

// Accept implements the Transporter interface.
func (t *TCPTransport) Accept(s *Server) {
	l, err := net.Listen("tcp", t.bindAddr)
	if err != nil {
		t.logger.Log("msg", "listen error", "err", err)
		return
	}

	t.listener = l

	for {
		conn, err := l.Accept()
		if err != nil {
			t.logger.Log("msg", "accept error", "err", err)
			continue
		}
		go t.handleConn(s, conn)
	}
}

func (t *TCPTransport) handleConn(s *Server, conn net.Conn) {
	s.register <- &TCPPeer{conn}
}

// Close implements the Transporter interface.
func (t *TCPTransport) Close() {
	t.listener.Close()
}

// Proto implements the Transporter interface.
func (t *TCPTransport) Proto() string {
	return "tcp"
}
