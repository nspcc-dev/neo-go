package network

import (
	"net"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

// Peer represents a network node neo-go is connected to.
type Peer interface {
	NetAddr() *net.TCPAddr
	Disconnect(error)
	WriteMsg(msg *Message) error
	Done() chan error
	Version() *payload.Version
	Handshaked() bool
	SendVersion(*Message) error
	SendVersionAck(*Message) error
	HandleVersion(*payload.Version) error
	HandleVersionAck() error
}
