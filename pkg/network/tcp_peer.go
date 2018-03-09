package network

import (
	"bytes"
	"net"
	"os"
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

	// Done is used to broadcast this peer has stopped running
	// and should be removed as reference.
	done chan struct{}
	send chan *Message

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
		send:        make(chan *Message),
		logger:      logger,
		connectedAt: time.Now().UTC(),
		handleProto: fun,
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
	p.send <- msg
}

// Done implemnets the Peer interface.
func (p *TCPPeer) Done() chan struct{} {
	return p.done
}

func (p *TCPPeer) run() error {
	errCh := make(chan error, 1)

	go p.readLoop(errCh)
	go p.writeLoop(errCh)

	err := <-errCh
	p.logger.Log("err", err)
	p.cleanup()
	return err
}

func (p *TCPPeer) readLoop(errCh chan error) {
	for {
		msg := &Message{}
		if err := msg.decode(p.conn); err != nil {
			errCh <- err
			break
		}
		p.handleMessage(msg)
	}
}

func (p *TCPPeer) writeLoop(errCh chan error) {
	buf := new(bytes.Buffer)

	for {
		msg := <-p.send
		if err := msg.encode(buf); err != nil {
			errCh <- err
			break
		}
		if _, err := p.conn.Write(buf.Bytes()); err != nil {
			errCh <- err
			break
		}
		buf.Reset()
	}
}

func (p *TCPPeer) cleanup() {
	p.conn.Close()
	close(p.send)
	p.done <- struct{}{}
}

func (p *TCPPeer) handleMessage(msg *Message) {
	switch msg.CommandType() {
	case CMDVersion:
		version := msg.Payload.(*payload.Version)
		p.version = version
		p.handleProto(msg, p)
	default:
		p.handleProto(msg, p)
	}
}
