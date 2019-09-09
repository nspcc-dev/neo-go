package network

import (
	"net"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

// TCPPeer represents a connected remote node in the
// network over TCP.
type TCPPeer struct {
	// underlying TCP connection.
	conn net.Conn
	addr net.TCPAddr

	// The version of the peer.
	version *payload.Version

	done chan error

	wg sync.WaitGroup
}

// NewTCPPeer returns a TCPPeer structure based on the given connection.
func NewTCPPeer(conn net.Conn) *TCPPeer {
	raddr := conn.RemoteAddr()
	// can't fail because raddr is a real connection
	tcpaddr, _ := net.ResolveTCPAddr(raddr.Network(), raddr.String())
	return &TCPPeer{
		conn: conn,
		done: make(chan error, 1),
		addr: *tcpaddr,
	}
}

// WriteMsg implements the Peer interface. This will write/encode the message
// to the underlying connection.
func (p *TCPPeer) WriteMsg(msg *Message) error {
	select {
	case err := <-p.done:
		return err
	default:
		return msg.Encode(p.conn)
	}
}

// NetAddr implements the Peer interface.
func (p *TCPPeer) NetAddr() *net.TCPAddr {
	return &p.addr
}

// Done implements the Peer interface and notifies
// all other resources operating on it that this peer
// is no longer running.
func (p *TCPPeer) Done() chan error {
	return p.done
}

// Disconnect will fill the peer's done channel with the given error.
func (p *TCPPeer) Disconnect(err error) {
	p.done <- err
}

// Version implements the Peer interface.
func (p *TCPPeer) Version() *payload.Version {
	return p.version
}

// SetVersion implements the Peer interface.
func (p *TCPPeer) SetVersion(v *payload.Version) {
	p.version = v
}
