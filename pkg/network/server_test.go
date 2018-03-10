package network

import (
	"os"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
)

func TestRegisterPeer(t *testing.T) {
	s := newTestServer()
	go s.run()

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
	go s.run()

	peer := newTestPeer()
	s.register <- peer
	s.register <- newTestPeer()
	s.register <- newTestPeer()
	assert.Equal(t, 3, s.PeerCount())

	s.unregister <- peerDrop{peer, nil}
	assert.Equal(t, 2, s.PeerCount())
}

type testNode struct{}

func (t testNode) version() *payload.Version {
	return &payload.Version{}
}

func (t testNode) handleProto(msg *Message, p Peer) error {
	return nil
}

func newTestServer() *Server {
	return &Server{
		logger:        log.NewLogfmtLogger(os.Stderr),
		id:            util.RandUint32(1000000, 9999999),
		quit:          make(chan struct{}, 1),
		register:      make(chan Peer),
		unregister:    make(chan peerDrop),
		badAddrOp:     make(chan func(map[string]bool)),
		badAddrOpDone: make(chan struct{}),
		peerOp:        make(chan func(map[Peer]bool)),
		peerOpDone:    make(chan struct{}),
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

func (p testPeer) Disconnect(err error) {

}
