package p2p

import (
	"net"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

type Peer interface {
	Endpoint() net.Addr
	Disconnect(error)
	Send(msg *Message)
	Done() chan struct{}
	Version() *payload.Version
}
