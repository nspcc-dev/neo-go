package network

import (
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

type testChain struct{}

func (chain testChain) AddHeaders(...*core.Header) error {
	return nil
}
func (chain testChain) AddBlock(*core.Block) error {
	return nil
}
func (chain testChain) BlockHeight() uint32 {
	return 0
}
func (chain testChain) HeaderHeight() uint32 {
	return 0
}
func (chain testChain) GetBlock(hash util.Uint256) (*core.Block, error) {
	return nil, nil
}
func (chain testChain) GetHeaderHash(int) util.Uint256 {
	return util.Uint256{}
}
func (chain testChain) CurrentHeaderHash() util.Uint256 {
	return util.Uint256{}
}
func (chain testChain) CurrentBlockHash() util.Uint256 {
	return util.Uint256{}
}
func (chain testChain) HasBlock(util.Uint256) bool {
	return false
}
func (chain testChain) HasTransaction(util.Uint256) bool {
	return false
}

type testDiscovery struct{}

func (d testDiscovery) BackFill(addrs ...string)   {}
func (d testDiscovery) PoolCount() int             { return 0 }
func (d testDiscovery) RegisterBadAddr(string)     {}
func (d testDiscovery) UnconnectedPeers() []string { return []string{} }
func (d testDiscovery) RequestRemote(n int)        {}
func (d testDiscovery) BadPeers() []string         { return []string{} }

type localTransport struct{}

func (t localTransport) Dial(addr string, timeout time.Duration) error {
	return nil
}
func (t localTransport) Accept()       {}
func (t localTransport) Proto() string { return "local" }
func (t localTransport) Close()        {}

var defaultMessageHandler = func(t *testing.T, msg *Message) {}

type localPeer struct {
	endpoint       util.Endpoint
	version        *payload.Version
	t              *testing.T
	messageHandler func(t *testing.T, msg *Message)
}

func newLocalPeer(t *testing.T) *localPeer {
	return &localPeer{
		t:              t,
		endpoint:       util.NewEndpoint("0.0.0.0:0"),
		messageHandler: defaultMessageHandler,
	}
}

func (p *localPeer) Endpoint() util.Endpoint {
	return p.endpoint
}
func (p *localPeer) Disconnect(err error) {}
func (p *localPeer) WriteMsg(msg *Message) error {
	p.messageHandler(p.t, msg)
	return nil
}
func (p *localPeer) Done() chan error {
	done := make(chan error)
	return done
}
func (p *localPeer) Version() *payload.Version {
	return p.version
}
func (p *localPeer) SetVersion(v *payload.Version) {
	p.version = v
}

func newTestServer() *Server {
	return &Server{
		ServerConfig: ServerConfig{},
		chain:        testChain{},
		transport:    localTransport{},
		discovery:    testDiscovery{},
		id:           util.RandUint32(1000000, 9999999),
		quit:         make(chan struct{}),
		register:     make(chan Peer),
		unregister:   make(chan peerDrop),
		peers:        make(map[Peer]bool),
	}

}
