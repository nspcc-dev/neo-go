package network

import (
	"net"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// TCPPeer represents a connected remote node in the
// network over TCP.
type TCPPeer struct {
	// underlying TCP connection.
	conn     net.Conn
	endpoint util.Endpoint

	// The version of the peer.
	version *payload.Version

	done chan error

	wg sync.WaitGroup
}

func NewTCPPeer(conn net.Conn) *TCPPeer {
	return &TCPPeer{
		conn:     conn,
		done:     make(chan error, 1),
		endpoint: util.NewEndpoint(conn.RemoteAddr().String()),
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

// Endpoint implements the Peer interface.
func (p *TCPPeer) Endpoint() util.Endpoint {
	return p.endpoint
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
