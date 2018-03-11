package p2p

import "net"

// TCPPeer represents a connected remote node in the
// network over TCP.
type TCPPeer struct {
	conn net.Conn
}

// Endpoint implements the Peer interface.
func (p *TCPPeer) Endpoint() net.Addr {
	return p.conn.RemoteAddr()
}

func (p *TCPPeer) Disconnect() {}
