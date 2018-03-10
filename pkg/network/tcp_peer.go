package network

import (
	"bytes"
	"net"
	"os"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/go-kit/kit/log"
)

// TCPPeer represents a connected remote node in the
// network over TCP.
type TCPPeer struct {
	// The endpoint of the peer.
	endpoint util.Endpoint

	// underlying connection.
	conn net.Conn

	// The version the peer declared when connecting.
	version *payload.Version

	// connectedAt is the timestamp the peer connected to
	// the network.
	connectedAt time.Time

	// handleProto is the handler that will handle the
	// incoming message along with its peer.
	handleProto protoHandleFunc

	// Done is used to broadcast that this peer has stopped running
	// and should be removed as reference.
	done chan struct{}

	// Every send to this channel will terminate the Peer.
	discErr chan error
	closed  chan struct{}
	wg      sync.WaitGroup

	logger log.Logger
}

// NewTCPPeer creates a new peer from a TCP connection.
func NewTCPPeer(conn net.Conn, fun protoHandleFunc) *TCPPeer {
	e := util.NewEndpoint(conn.RemoteAddr().String())
	logger := log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "component", "peer", "endpoint", e)

	return &TCPPeer{
		endpoint:    e,
		conn:        conn,
		done:        make(chan struct{}),
		logger:      logger,
		connectedAt: time.Now().UTC(),
		handleProto: fun,
		discErr:     make(chan error),
		closed:      make(chan struct{}),
	}
}

// Version implements the Peer interface.
func (p *TCPPeer) Version() *payload.Version {
	return p.version
}

// Endpoint implements the Peer interface.
func (p *TCPPeer) Endpoint() util.Endpoint {
	return p.endpoint
}

// Send implements the Peer interface.
func (p *TCPPeer) Send(msg *Message) {
	buf := new(bytes.Buffer)
	if err := msg.encode(buf); err != nil {
		p.discErr <- err
		return
	}
	if _, err := p.conn.Write(buf.Bytes()); err != nil {
		p.discErr <- err
		return
	}
}

// Done implemnets the Peer interface. It use is to
// notify the Node that this peer is no longer available
// for sending messages to.
func (p *TCPPeer) Done() chan struct{} {
	return p.done
}

// Disconnect terminates the peer connection.
func (p *TCPPeer) Disconnect(err error) {
	select {
	case p.discErr <- err:
	case <-p.closed:
	}
}

func (p *TCPPeer) run() (err error) {
	p.wg.Add(1)
	go p.readLoop()

run:
	for {
		select {
		case err = <-p.discErr:
			break run
		}
	}

	p.conn.Close()
	close(p.closed)
	// Close done instead of sending empty struct.
	// It could happen that startProtocol in Node never happens
	// on connection errors for example.
	close(p.done)
	p.wg.Wait()
	return err
}

func (p *TCPPeer) readLoop() {
	defer p.wg.Done()
	for {
		select {
		case <-p.closed:
			return
		default:
			msg := &Message{}
			if err := msg.decode(p.conn); err != nil {
				p.discErr <- err
				return
			}
			p.handleMessage(msg)
		}
	}
}

func (p *TCPPeer) handleMessage(msg *Message) {
	switch msg.CommandType() {
	case CMDVersion:
		version := msg.Payload.(*payload.Version)
		p.version = version
		fallthrough
	default:
		if err := p.handleProto(msg, p); err != nil {
			p.discErr <- err
		}
	}
}
