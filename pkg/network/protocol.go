package network

import (
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

// A ProtoHandler is an interface that abstract the implementation
// of the NEO protocol.
type ProtoHandler interface {
	version() *payload.Version
	handleProto(*Message, Peer)
}

type protoHandleFunc func(*Message, Peer)

// Noder is anything that implements the NEO protocol
// and can return the Blockchain object.
type Noder interface {
	ProtoHandler
	blockchain() *core.Blockchain
}
