package network

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
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

	server *Server

	// Done is used to broadcast this peer has stopped running
	// and should be removed as reference.
	done   chan struct{}
	send   chan *Message
	logger *log.Logger
}

// NewTCPPeer creates a new peer from a TCP connection.
func NewTCPPeer(conn net.Conn, s *Server) *TCPPeer {
	e := util.NewEndpoint(conn.RemoteAddr().String())
	pre := fmt.Sprintf("[%s] ", e)
	return &TCPPeer{
		endpoint:    e,
		conn:        conn,
		done:        make(chan struct{}),
		send:        make(chan *Message),
		server:      s,
		logger:      log.New(os.Stdout, pre, 0),
		connectedAt: time.Now().UTC(),
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

	return <-errCh
}

func (p *TCPPeer) readLoop(errCh chan error) {
	for {
		msg := &Message{}
		if err := msg.decode(p.conn); err != nil {
			errCh <- err
			break
		}
		if err := p.handleMessage(msg); err != nil {
			errCh <- err
			break
		}
	}
	p.cleanup()
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
	p.cleanup()
}

func (p *TCPPeer) cleanup() {
	p.conn.Close()
	p.done <- struct{}{}
}

func (p *TCPPeer) handleMessage(msg *Message) error {
	cmd := msg.CommandType()

	switch cmd {
	case CMDVersion:
		version := msg.Payload.(*payload.Version)
		p.version = version
		return p.server.proto.handleVersionCmd(version, p)
	case CMDAddr:
		addressList := msg.Payload.(*payload.AddressList)
		return p.server.proto.handleAddrCmd(addressList, p)
	case CMDInv:
		inventory := msg.Payload.(*payload.Inventory)
		return p.server.proto.handleInvCmd(inventory, p)
	case CMDBlock:
		block := msg.Payload.(*core.Block)
		return p.server.proto.handleBlockCmd(block, p)
	case CMDVerack:
		// Only start the protocol if we got the version and verack
		// received.
		if p.version != nil {
			go p.server.proto.startProtocol(p)
		}
		return nil
	case CMDUnknown:
		return errors.New("received non-protocol messgae")
	}

	return nil
}
