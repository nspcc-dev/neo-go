package network

import (
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

type Peer interface {
	Endpoint() util.Endpoint
	Disconnect(error)
	Send(msg *Message)
	Done() chan error
	Version() *payload.Version
}
