package network

import (
	"net"

	"github.com/nspcc-dev/neo-go/pkg/network/payload"
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
	EnqueuePacket(bool, []byte) error

	// EnqueueP2PMessage is a temporary wrapper that sends a message via
	// EnqueueP2PPacket if there is no error in serializing it.
	EnqueueP2PMessage(*Message) error

	// EnqueueP2PPacket is a blocking packet enqueuer, it doesn't return until
	// it puts given packet into the queue. It accepts a slice of bytes that
	// can be shared with other queues (so that message marshalling can be
	// done once for all peers). Does nothing is the peer is not yet
	// completed handshaking. This queue is intended to be used for unicast
	// peer to peer communication that is more important than broadcasts
	// (handled by EnqueuePacket), but less important than high-priority
	// messages (handled by EnqueueHPPacket).
	EnqueueP2PPacket([]byte) error

	// EnqueueHPPacket is a blocking high priority packet enqueuer, it
	// doesn't return until it puts given packet into the high-priority
	// queue.
	EnqueueHPPacket(bool, []byte) error
	Version() *payload.Version
	LastBlockIndex() uint32
	Handshaked() bool
	IsFullNode() bool

	// SendPing enqueues a ping message to be sent to the peer and does
	// appropriate protocol handling like timeouts and outstanding pings
	// management.
	SendPing(*Message) error
	// SendVersion checks handshake status and sends a version message to
	// the peer.
	SendVersion() error
	SendVersionAck(*Message) error
	// StartProtocol is a goroutine to be run after the handshake. It
	// implements basic peer-related protocol handling.
	StartProtocol()
	HandleVersion(*payload.Version) error
	HandleVersionAck() error

	// HandlePing checks ping contents against Peer's state and updates it.
	HandlePing(ping *payload.Ping) error

	// HandlePong checks pong contents against Peer's state and updates it.
	HandlePong(pong *payload.Ping) error

	// AddGetAddrSent is to inform local peer context that a getaddr command
	// is sent. The decision to send getaddr is server-wide, but it needs to be
	// accounted for in peer's context, thus this method.
	AddGetAddrSent()

	// CanProcessAddr checks whether an addr command is expected to come from
	// this peer and can be processed.
	CanProcessAddr() bool
}
