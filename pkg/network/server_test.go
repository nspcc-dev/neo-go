package network

import (
	"log"
	"os"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestRegisterPeer(t *testing.T) {
	s := newTestServer()
	go s.loop()

	assert.NotZero(t, s.id)
	assert.Zero(t, s.PeerCount())

	lenPeers := 10
	for i := 0; i < lenPeers; i++ {
		s.register <- newTestPeer()
	}
	assert.Equal(t, lenPeers, s.PeerCount())
}

func TestUnregisterPeer(t *testing.T) {
	s := newTestServer()
	go s.loop()

	peer := newTestPeer()
	s.register <- peer
	s.register <- newTestPeer()
	s.register <- newTestPeer()
	assert.Equal(t, 3, s.PeerCount())

	s.unregister <- peer
	assert.Equal(t, 2, s.PeerCount())
}

type testNode struct{}

func (t testNode) version() *payload.Version {
	return &payload.Version{}
}

func (t testNode) startProtocol(p Peer) {}

func (t testNode) handleVersionCmd(version *payload.Version, p Peer) error {
	return nil
}

func (t testNode) handleInvCmd(version *payload.Inventory, p Peer) error {
	return nil
}

func (t testNode) handleBlockCmd(version *core.Block, p Peer) error {
	return nil
}

func (t testNode) handleAddrCmd(version *payload.AddressList, p Peer) error {
	return nil
}

func newTestServer() *Server {
	return &Server{
		logger:        log.New(os.Stdout, "[NEO NODE] :: ", 0),
		id:            util.RandUint32(1000000, 9999999),
		quit:          make(chan struct{}, 1),
		register:      make(chan Peer),
		unregister:    make(chan Peer),
		badAddrOp:     make(chan func(map[string]bool)),
		badAddrOpDone: make(chan struct{}),
		peerOp:        make(chan func(map[Peer]bool)),
		peerOpDone:    make(chan struct{}),
		peers:         map[Peer]bool{},
		badAddrs:      map[string]bool{},
		proto:         testNode{},
	}
}

type testPeer struct {
	done chan struct{}
}

func newTestPeer() testPeer {
	return testPeer{
		done: make(chan struct{}),
	}
}

func (p testPeer) Version() *payload.Version {
	return &payload.Version{}
}

func (p testPeer) Endpoint() util.Endpoint {
	return util.Endpoint{}
}

func (p testPeer) Send(msg *Message) {}

func (p testPeer) Done() chan struct{} {
	return p.done
}
