package network

import (
	"context"
	"net"

	"github.com/nspcc-dev/neo-go/pkg/network/payload"
)

// Peer represents a network node neo-go is connected to.
type Peer interface {
	// RemoteAddr returns the remote address that we're connected to now.
	RemoteAddr() net.Addr
	// PeerAddr returns the remote address that should be used to establish
	// a new connection to the node. It can differ from the RemoteAddr
	// address in case the remote node is a client and its current
	// connection port is different from the one the other node should use
	// to connect to it. It's only valid after the handshake is completed.
	// Before that, it returns the same address as RemoteAddr.
	PeerAddr() net.Addr
	Disconnect(error)

	// BroadcastPacket is a context-bound packet enqueuer, it either puts the
	// given packet into the queue or exits with errors if the context expires
	// or peer disconnects. It accepts a slice of bytes that
	// can be shared with other queues (so that message marshalling can be
	// done once for all peers). It returns an error if the peer has not yet
	// completed handshaking.
	BroadcastPacket(context.Context, []byte) error

	// BroadcastHPPacket is the same as BroadcastPacket, but uses a high-priority
	// queue.
	BroadcastHPPacket(context.Context, []byte) error

	// EnqueueP2PMessage is a blocking packet enqueuer, it doesn't return until
	// it puts the given message into the queue. It returns an error if the peer
	// has not yet completed handshaking. This queue is intended to be used for
	// unicast peer to peer communication that is more important than broadcasts
	// (handled by BroadcastPacket) but less important than high-priority
	// messages (handled by EnqueueHPMessage).
	EnqueueP2PMessage(*Message) error

	// EnqueueHPMessage is similar to EnqueueP2PMessage, but uses a high-priority
	// queue.
	EnqueueHPMessage(*Message) error
	Version() *payload.Version
	LastBlockIndex() uint32
	Handshaked() bool
	IsFullNode() bool

	// SetPingTimer adds an outgoing ping to the counter and sets a PingTimeout
	// timer that will shut the connection down in case of no response.
	SetPingTimer()
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
