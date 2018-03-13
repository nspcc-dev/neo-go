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

	done   chan struct{}
	closed chan struct{}
	disc   chan error

	wg sync.WaitGroup
}

func NewTCPPeer(conn net.Conn, proto chan protoTuple) *TCPPeer {
	return &TCPPeer{
		conn:     conn,
		done:     make(chan struct{}),
		closed:   make(chan struct{}),
		disc:     make(chan error),
		endpoint: util.NewEndpoint(conn.RemoteAddr().String()),
	}
}

// Send implements the Peer interface. This will encode the message
// to the underlying connection. The server will handle the error
// returned from Send and will disconnect the peer.
func (p *TCPPeer) Send(msg *Message) error {
	return msg.encode(p.conn)
}

// Endpoint implements the Peer interface.
func (p *TCPPeer) Endpoint() util.Endpoint {
	return p.endpoint
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

	// If the peer has not started the protocol with the server
	// there will be noone reading from this channel.
	select {
	case p.done <- struct{}{}:
	default:
	}

	close(p.closed)
	p.conn.Close()
	p.wg.Wait()
	return
}

// Disconnect implements the Peer interface.
func (p *TCPPeer) Disconnect(reason error) {
	p.disc <- reason
}
