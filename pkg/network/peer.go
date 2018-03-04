package network

import (
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Peer is the local representation of a remote node. It's an interface that may
// be backed by any concrete transport: local, HTTP, tcp.
type Peer interface {
	id() uint32
	addr() util.Endpoint
	disconnect()
	Send(*Message) error
	version() *payload.Version
}

// LocalPeer is the simplest kind of peer, mapped to a server in the
// same process-space.
type LocalPeer struct {
	s        *Server
	nonce    uint32
	endpoint util.Endpoint
	pVersion *payload.Version
}

// NewLocalPeer return a LocalPeer.
func NewLocalPeer(s *Server) *LocalPeer {
	e, _ := util.EndpointFromString("1.1.1.1:1111")
	return &LocalPeer{endpoint: e, s: s}
}

func (p *LocalPeer) Send(msg *Message) error {
	switch msg.commandType() {
	case cmdVersion:
		version := msg.Payload.(*payload.Version)
		return p.s.handleVersionCmd(version, p)
	case cmdGetAddr:
		return p.s.handleGetaddrCmd(msg, p)
	default:
		return nil
	}
}

// Version implements the Peer interface.
func (p *LocalPeer) version() *payload.Version {
	return p.pVersion
}

func (p *LocalPeer) id() uint32          { return p.nonce }
func (p *LocalPeer) addr() util.Endpoint { return p.endpoint }
func (p *LocalPeer) disconnect()         {}
