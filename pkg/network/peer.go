package network

import (
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// A Peer is the local representation of a remote peer.
// It's an interface that may be backed by any concrete
// transport.
type Peer interface {
	Version() *payload.Version
	Endpoint() util.Endpoint
	Send(*Message)
	Done() chan struct{}
}
