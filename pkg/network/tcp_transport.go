package network

import (
	"errors"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TCPTransport allows network communication over TCP.
type TCPTransport struct {
	log      *zap.Logger
	server   *Server
	listener net.Listener
	bindAddr string
	lock     sync.RWMutex
	quit     bool
}

// NewTCPTransport returns a new TCPTransport that will listen for
// new incoming peer connections.
func NewTCPTransport(s *Server, bindAddr string, log *zap.Logger) *TCPTransport {
	return &TCPTransport{
		log:      log,
		server:   s,
		bindAddr: bindAddr,
	}
}

// Dial implements the Transporter interface.
func (t *TCPTransport) Dial(addr string, timeout time.Duration) (AddressablePeer, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	p := NewTCPPeer(conn, addr, t.server)
	go p.handleConn()
	return p, nil
}

// Accept implements the Transporter interface.
func (t *TCPTransport) Accept() {
	l, err := net.Listen("tcp", t.bindAddr)
	if err != nil {
		t.log.Panic("TCP listen error", zap.Error(err))
		return
	}

	t.lock.Lock()
	if t.quit {
		t.lock.Unlock()
		l.Close()
		return
	}
	t.listener = l
	t.lock.Unlock()

	for {
		conn, err := l.Accept()
		if err != nil {
			t.lock.Lock()
			quit := t.quit
			t.lock.Unlock()
			if errors.Is(err, net.ErrClosed) && quit {
				break
			}
			t.log.Warn("TCP accept error", zap.Error(err))
			continue
		}
		p := NewTCPPeer(conn, "", t.server)
		go p.handleConn()
	}
}

// Close implements the Transporter interface.
func (t *TCPTransport) Close() {
	t.lock.Lock()
	if t.listener != nil {
		t.listener.Close()
	}
	t.quit = true
	t.lock.Unlock()
}

// Proto implements the Transporter interface.
func (t *TCPTransport) Proto() string {
	return "tcp"
}

// Address implements the Transporter interface.
func (t *TCPTransport) Address() string {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if t.listener != nil {
		return t.listener.Addr().String()
	}
	return ""
}
