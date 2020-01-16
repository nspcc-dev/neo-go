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

	// EnqueueMessage is a temporary wrapper that sends a message via
	// EnqueuePacket if there is no error in serializing it.
	EnqueueMessage(*Message) error

	// EnqueuePacket is a blocking packet enqueuer, it doesn't return until
	// it puts given packet into the queue. It accepts a slice of bytes that
	// can be shared with other queues (so that message marshalling can be
	// done once for all peers). Does nothing is the peer is not yet
	// completed handshaking.
	EnqueuePacket([]byte) error

	// EnqueueHPPacket is a blocking high priority packet enqueuer, it
	// doesn't return until it puts given packet into the high-priority
	// queue.
	EnqueueHPPacket([]byte) error
	Version() *payload.Version
	LastBlockIndex() uint32
	UpdateLastBlockIndex(lbIndex uint32)
	Handshaked() bool
	SendVersion(*Message) error
	SendVersionAck(*Message) error
	// StartProtocol is a goroutine to be run after the handshake. It
	// implements basic peer-related protocol handling.
	StartProtocol()
	HandleVersion(*payload.Version) error
	HandleVersionAck() error
	GetPingSent() int
	UpdatePingSent(int)
}
