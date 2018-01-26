package network

import (
	"log"
	"net"
)

// Peer represents a remote node, backed by TCP transport.
type Peer struct {
	// underlaying TCP connection
	conn net.Conn
	// channel to coordinate message writes back to the connection.
	send chan *Message
	// verack is true if this node has sended it's version.
	verack bool
	// whether we or him made the initial connection.
	initiator bool
}

// NewPeer returns a (TCP) Peer.
func NewPeer(conn net.Conn, init bool) *Peer {
	return &Peer{
		conn:      conn,
		send:      make(chan *Message),
		initiator: init,
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
		rpcLogger.Printf("OUT :: %+v", msg)
		if err := msg.encode(p.conn); err != nil {
			log.Printf("encode error: %s", err)
		}
	}
}
