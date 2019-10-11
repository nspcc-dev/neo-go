package network

import (
	"math/rand"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

type testChain struct {
	blockheight uint32
}

func (chain testChain) GetConfig() config.ProtocolConfiguration {
	panic("TODO")
}

func (chain testChain) References(t *transaction.Transaction) map[transaction.Input]*transaction.Output {
	panic("TODO")
}

func (chain testChain) FeePerByte(t *transaction.Transaction) util.Fixed8 {
	panic("TODO")
}

func (chain testChain) SystemFee(t *transaction.Transaction) util.Fixed8 {
	panic("TODO")
}

func (chain testChain) NetworkFee(t *transaction.Transaction) util.Fixed8 {
	panic("TODO")
}

func (chain testChain) AddHeaders(...*core.Header) error {
	panic("TODO")
}
func (chain *testChain) AddBlock(block *core.Block) error {
	if block.Index == chain.blockheight+1 {
		atomic.StoreUint32(&chain.blockheight, block.Index)
	}
	return nil
}
func (chain *testChain) BlockHeight() uint32 {
	return atomic.LoadUint32(&chain.blockheight)
}
func (chain testChain) HeaderHeight() uint32 {
	return 0
}
func (chain testChain) GetBlock(hash util.Uint256) (*core.Block, error) {
	panic("TODO")
}
func (chain testChain) GetContractState(hash util.Uint160) *core.ContractState {
	panic("TODO")
}
func (chain testChain) GetHeaderHash(int) util.Uint256 {
	return util.Uint256{}
}
func (chain testChain) GetHeader(hash util.Uint256) (*core.Header, error) {
	panic("TODO")
}

func (chain testChain) GetAssetState(util.Uint256) *core.AssetState {
	panic("TODO")
}
func (chain testChain) GetAccountState(util.Uint160) *core.AccountState {
	panic("TODO")
}
func (chain testChain) GetScriptHashesForVerifying(*transaction.Transaction) ([]util.Uint160, error) {
	panic("TODO")
}
func (chain testChain) GetStorageItem(scripthash util.Uint160, key []byte) *core.StorageItem {
	panic("TODO")
}
func (chain testChain) GetStorageItems(hash util.Uint160) (map[string]*core.StorageItem, error) {
	panic("TODO")
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
func (chain testChain) GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error) {
	panic("TODO")
}

func (chain testChain) GetUnspentCoinState(util.Uint256) *core.UnspentCoinState {
	panic("TODO")
}

func (chain testChain) GetMemPool() core.MemPool {
	panic("TODO")
}

func (chain testChain) IsLowPriority(*transaction.Transaction) bool {
	panic("TODO")
}

func (chain testChain) Verify(*transaction.Transaction) error {
	panic("TODO")
}

type testDiscovery struct{}

func (d testDiscovery) BackFill(addrs ...string)       {}
func (d testDiscovery) PoolCount() int                 { return 0 }
func (d testDiscovery) RegisterBadAddr(string)         {}
func (d testDiscovery) RegisterGoodAddr(string)        {}
func (d testDiscovery) UnregisterConnectedAddr(string) {}
func (d testDiscovery) UnconnectedPeers() []string     { return []string{} }
func (d testDiscovery) RequestRemote(n int)            {}
func (d testDiscovery) BadPeers() []string             { return []string{} }
func (d testDiscovery) GoodPeers() []string            { return []string{} }

type localTransport struct{}

func (t localTransport) Dial(addr string, timeout time.Duration) error {
	return nil
}
func (t localTransport) Accept()       {}
func (t localTransport) Proto() string { return "local" }
func (t localTransport) Close()        {}

var defaultMessageHandler = func(t *testing.T, msg *Message) {}

type localPeer struct {
	netaddr        net.TCPAddr
	version        *payload.Version
	handshaked     bool
	t              *testing.T
	messageHandler func(t *testing.T, msg *Message)
}

func newLocalPeer(t *testing.T) *localPeer {
	naddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	return &localPeer{
		t:              t,
		netaddr:        *naddr,
		messageHandler: defaultMessageHandler,
	}
}

func (p *localPeer) NetAddr() *net.TCPAddr {
	return &p.netaddr
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
func (p *localPeer) HandleVersion(v *payload.Version) error {
	p.version = v
	return nil
}
func (p *localPeer) SendVersion(m *Message) error {
	return p.WriteMsg(m)
}
func (p *localPeer) SendVersionAck(m *Message) error {
	return p.WriteMsg(m)
}
func (p *localPeer) HandleVersionAck() error {
	p.handshaked = true
	return nil
}

func (p *localPeer) Handshaked() bool {
	return p.handshaked
}

func newTestServer() *Server {
	return &Server{
		ServerConfig: ServerConfig{},
		chain:        &testChain{},
		transport:    localTransport{},
		discovery:    testDiscovery{},
		id:           rand.Uint32(),
		quit:         make(chan struct{}),
		register:     make(chan Peer),
		unregister:   make(chan peerDrop),
		peers:        make(map[Peer]bool),
	}

}
