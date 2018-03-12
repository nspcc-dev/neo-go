package p2p

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
	addr net.Addr

	// The version of the peer.
	version *payload.Version

	done   chan struct{}
	closed chan struct{}
	disc   chan error

	wg sync.WaitGroup
}

func NewTCPPeer(conn net.Conn, proto chan protoTuple) *TCPPeer {
	return &TCPPeer{
		conn:   conn,
		closed: make(chan struct{}),
		done:   make(chan struct{}),
		disc:   make(chan error),
		addr:   conn.RemoteAddr(),
	}
}

// Send implements the Peer interface. This will encode the message
// to the underlying connection. The peer will disconnect if encode
// failed.
func (p *TCPPeer) Send(msg *Message) {
	if err := msg.encode(p.conn); err != nil {
		select {
		case p.disc <- err:
		case <-p.closed:
		}
	}
}

// Endpoint implements the Peer interface.
func (p *TCPPeer) Endpoint() net.Addr {
	return p.addr
}

// Done implements the Peer interface and notifies
// all other resources operating on it to release this
// peer.
func (p *TCPPeer) Done() chan struct{} {
	return p.done
}

// Version implements the Peer interface.
func (p *TCPPeer) Version() *payload.Version {
	return p.version
}

func (p *TCPPeer) readLoop(proto chan protoTuple, readErr chan error) {
	defer p.wg.Done()
	for {
		select {
		case <-p.closed:
			return
		default:
			msg := &Message{}
			if err := msg.decode(p.conn); err != nil {
				readErr <- err
				return
			}
			p.handleMessage(msg, proto)
		}
	}
}

func (p *TCPPeer) handleMessage(msg *Message, proto chan protoTuple) {
	switch payload := msg.Payload.(type) {
	case *payload.Version:
		p.version = payload
	}
	proto <- protoTuple{
		msg:  msg,
		peer: p,
	}
}

func (p *TCPPeer) run(proto chan protoTuple) (err error) {
	readErr := make(chan error, 1)
	p.wg.Add(1)
	go p.readLoop(proto, readErr)

run:
	for {
		select {
		case err = <-p.disc:
			break run
		case err = <-readErr:
			break run
		}
	}

	close(p.closed)
	close(p.done)
	p.conn.Close()
	p.wg.Wait()
	return
}

// Disconnect implements the Peer interface.
func (p *TCPPeer) Disconnect(reason error) {}
