package network

import (
	"net"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

// Peer represents a network node neo-go is connected to.
type Peer interface {
	// RemoteAddr returns the remote address that we're connected to now.
	RemoteAddr() net.Addr
	// PeerAddr returns the remote address that should be used to establish
	// a new connection to the node. It can differ from the RemoteAddr
	// address in case where the remote node is a client and its current
	// connection port is different from the one the other node should use
	// to connect to it. It's only valid after the handshake is completed,
	// before that it returns the same address as RemoteAddr.
	PeerAddr() net.Addr
	Disconnect(error)
	WriteMsg(msg *Message) error
	Done() chan error
	Version() *payload.Version
	LastBlockIndex() uint32
	UpdateLastBlockIndex(lbIndex uint32)
	Handshaked() bool
	SendVersion(*Message) error
	SendVersionAck(*Message) error
	HandleVersion(*payload.Version) error
	HandleVersionAck() error
	GetPingSent() int
	UpdatePingSent(int)
}
