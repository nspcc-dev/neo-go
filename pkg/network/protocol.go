package network

import (
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

// A ProtoHandler is interface that abstract the implementation
// of the NEO protocol.
type ProtoHandler interface {
	version() *payload.Version
	startProtocol(Peer)
	handleVersionCmd(*payload.Version, Peer) error
	handleInvCmd(*payload.Inventory, Peer) error
	handleBlockCmd(*core.Block, Peer) error
	handleAddrCmd(*payload.AddressList, Peer) error
}
