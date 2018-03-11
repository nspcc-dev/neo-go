package p2p

import "net"

type Peer interface {
	Endpoint() net.Addr
	Disconnect()
}
