package network

import (
	"net"

	"github.com/anthdm/neo-go/pkg/util"
)

// Peer is the local representation of a remote node. It's an interface that may
// be backed by any concrete transport: local, HTTP, tcp.
type Peer interface {
	id() uint32
	addr() util.Endpoint
	verack() bool
	disconnect()
	callVersion(*Message)
	callGetaddr(*Message)
}

// LocalPeer is the simplest kind of peer, mapped to a server in the
// same process-space.
type LocalPeer struct {
	s        *Server
	nonce    uint32
	isVerack bool
	endpoint util.Endpoint
}

// NewLocalPeer return a LocalPeer.
func NewLocalPeer(s *Server) *LocalPeer {
	e, _ := util.EndpointFromString("1.1.1.1:1111")
	return &LocalPeer{endpoint: e, s: s}
}

func (p *LocalPeer) callVersion(msg *Message) {
	p.s.handleVersionCmd(msg, p)
}

func (p *LocalPeer) callGetaddr(msg *Message) {
	p.s.handleGetaddrCmd(msg, p)
}

func (p *LocalPeer) id() uint32          { return p.nonce }
func (p *LocalPeer) verack() bool        { return p.isVerack }
func (p *LocalPeer) addr() util.Endpoint { return p.endpoint }
func (p *LocalPeer) disconnect()         {}

// TCPPeer represents a remote node, backed by TCP transport.
type TCPPeer struct {
	s *Server
	// nonce (id) of the peer.
	nonce uint32
	// underlying TCP connection
	conn net.Conn
	// host and port information about this peer.
	endpoint util.Endpoint
	// channel to coordinate messages writen back to the connection.
	send chan *Message
	// whether this peers version was acknowledged.
	isVerack bool
}

// NewTCPPeer returns a pointer to a TCP Peer.
func NewTCPPeer(conn net.Conn, s *Server) *TCPPeer {
	e, _ := util.EndpointFromString(conn.RemoteAddr().String())

	return &TCPPeer{
		conn:     conn,
		send:     make(chan *Message),
		endpoint: e,
		s:        s,
	}
}

func (p *TCPPeer) callVersion(msg *Message) {
	p.send <- msg
}

// id implements the peer interface
func (p *TCPPeer) id() uint32 {
	return p.nonce
}

// endpoint implements the peer interface
func (p *TCPPeer) addr() util.Endpoint {
	return p.endpoint
}

// verack implements the peer interface
func (p *TCPPeer) verack() bool {
	return p.isVerack
}

// callGetaddr will send the "getaddr" command to the remote.
func (p *TCPPeer) callGetaddr(msg *Message) {
	p.send <- msg
}

func (p *TCPPeer) disconnect() {
	close(p.send)
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
		msg := <-p.send

		p.s.logger.Printf("OUT :: %s :: %+v", msg.commandType(), msg.Payload)

		// should we disconnect here?
		if err := msg.encode(p.conn); err != nil {
			p.s.logger.Printf("encode error: %s", err)
		}
	}
}
