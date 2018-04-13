package network

import (
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

type Peer interface {
	Endpoint() util.Endpoint
	Disconnect(error)
	WriteMsg(msg *Message) error
	Done() chan error
	Version() *payload.Version
	SetVersion(*payload.Version)
}
