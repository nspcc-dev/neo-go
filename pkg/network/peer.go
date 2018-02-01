package network

import (
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Peer is the local representation of a remote node. It's an interface that may
// be backed by any concrete transport: local, HTTP, tcp.
type Peer interface {
	id() uint32
	addr() util.Endpoint
	disconnect()
	callVersion(*Message)
	callGetaddr(*Message)
}

// LocalPeer is the simplest kind of peer, mapped to a server in the
// same process-space.
type LocalPeer struct {
	s        *Server
	nonce    uint32
	endpoint util.Endpoint
}

// NewLocalPeer return a LocalPeer.
func NewLocalPeer(s *Server) *LocalPeer {
	e, _ := util.EndpointFromString("1.1.1.1:1111")
	return &LocalPeer{endpoint: e, s: s}
}

func (p *LocalPeer) callVersion(msg *Message) {
	p.s.handleVersionCmd(msg, p)
}

func (p *LocalPeer) callGetaddr(msg *Message) {
	p.s.handleGetaddrCmd(msg, p)
}

func (p *LocalPeer) id() uint32          { return p.nonce }
func (p *LocalPeer) addr() util.Endpoint { return p.endpoint }
func (p *LocalPeer) disconnect()         {}
