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
	SetVersion(*payload.Version)
}
