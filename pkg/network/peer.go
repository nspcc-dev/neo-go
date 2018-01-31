package network

import (
	"fmt"
	"log"
	"net"

	"github.com/anthdm/neo-go/pkg/util"
)

// Peer is the local representation of a remote node. It's an interface that may
// be backed by any concrete transport: local, HTTP, tcp.
type Peer interface {
	id() uint32
	endpoint() util.Endpoint
	send(*Message)
	verack() bool
	verify(uint32)
	disconnect()
}

// LocalPeer is a peer without any transport, mainly used for testing.
type LocalPeer struct {
	_id       uint32
	_verack   bool
	_endpoint util.Endpoint
	_send     chan *Message
}

// NewLocalPeer return a LocalPeer.
func NewLocalPeer() *LocalPeer {
	e, _ := util.EndpointFromString("1.1.1.1:1111")
	return &LocalPeer{_endpoint: e}
}

func (p *LocalPeer) id() uint32              { return p._id }
func (p *LocalPeer) verack() bool            { return p._verack }
func (p *LocalPeer) endpoint() util.Endpoint { return p._endpoint }
func (p *LocalPeer) disconnect()             {}

func (p *LocalPeer) send(msg *Message) {
	p._send <- msg
}

func (p *LocalPeer) verify(id uint32) {
	fmt.Println(id)
	p._verack = true
	p._id = id
}

// TCPPeer represents a remote node, backed by TCP transport.
type TCPPeer struct {
	_id uint32
	// underlying TCP connection
	conn net.Conn
	// host and port information about this peer.
	_endpoint util.Endpoint
	// channel to coordinate messages writen back to the connection.
	_send chan *Message
	// whether this peers version was acknowledged.
	_verack bool
}

// NewTCPPeer returns a pointer to a TCP Peer.
func NewTCPPeer(conn net.Conn) *TCPPeer {
	e, _ := util.EndpointFromString(conn.RemoteAddr().String())

	return &TCPPeer{
		conn:      conn,
		_send:     make(chan *Message),
		_endpoint: e,
	}
}

// id implements the peer interface
func (p *TCPPeer) id() uint32 {
	return p._id
}

// endpoint implements the peer interface
func (p *TCPPeer) endpoint() util.Endpoint {
	return p._endpoint
}

// verack implements the peer interface
func (p *TCPPeer) verack() bool {
	return p._verack
}

// verify implements the peer interface
func (p *TCPPeer) verify(id uint32) {
	p._id = id
	p._verack = true
}

// send implements the peer interface
func (p *TCPPeer) send(msg *Message) {
	p._send <- msg
}

func (p *TCPPeer) disconnect() {
	close(p._send)
	p.conn.Close()
}

// writeLoop writes messages to the underlying TCP connection.
// A goroutine writeLoop is started for each connection.
// There should be at most one writer to a connection executing
// all writes from this goroutine.
func (p *TCPPeer) writeLoop() {
	// clean up the connection.
	defer func() {
		p.conn.Close()
	}()

	for {
		msg := <-p._send

		rpcLogger.Printf("[SERVER] :: OUT :: %s :: %+v", msg.commandType(), msg.Payload)

		if err := msg.encode(p.conn); err != nil {
			log.Printf("encode error: %s", err)
		}
	}
}
