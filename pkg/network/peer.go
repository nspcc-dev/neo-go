package network

import (
	"log"
	"net"

	"github.com/anthdm/neo-go/pkg/util"
)

// Peer represents a remote node, backed by TCP transport.
type Peer struct {
	id uint32
	// underlying TCP connection
	conn net.Conn
	// host and port information about this peer.
	endpoint util.Endpoint
	// channel to coordinate messages writen back to the connection.
	send chan *Message
	// whether this peers version was acknowledged.
	verack bool
}

// NewPeer returns a (TCP) Peer.
func NewPeer(conn net.Conn) *Peer {
	e, _ := util.EndpointFromString(conn.RemoteAddr().String())

	return &Peer{
		conn:     conn,
		send:     make(chan *Message),
		endpoint: e,
	}
}

// writeLoop writes messages to the underlying TCP connection.
// A goroutine writeLoop is started for each connection.
// There should be at most one writer to a connection executing
// all writes from this goroutine.
func (p *Peer) writeLoop() {
	// clean up the connection.
	defer func() {
		p.conn.Close()
	}()

	for {
		msg := <-p.send
		rpcLogger.Printf("OUT :: %s", msg.commandType())
		if err := msg.encode(p.conn); err != nil {
			log.Printf("encode error: %s", err)
		}
	}
}
