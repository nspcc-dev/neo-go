package network

import (
	"math/rand"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"go.uber.org/zap/zaptest"
)

type testChain struct {
	blockheight uint32
}

func (chain testChain) ApplyPolicyToTxSet([]mempool.TxWithFee) []mempool.TxWithFee {
	panic("TODO")
}
func (chain testChain) GetConfig() config.ProtocolConfiguration {
	panic("TODO")
}
func (chain testChain) CalculateClaimable(util.Fixed8, uint32, uint32) (util.Fixed8, util.Fixed8, error) {
	panic("TODO")
}

func (chain testChain) References(t *transaction.Transaction) ([]transaction.InOut, error) {
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

func (chain testChain) AddHeaders(...*block.Header) error {
	panic("TODO")
}
func (chain *testChain) AddBlock(block *block.Block) error {
	if block.Index == chain.blockheight+1 {
		atomic.StoreUint32(&chain.blockheight, block.Index)
	}
	return nil
}
func (chain *testChain) AddStateRoot(r *state.MPTRoot) error {
	panic("TODO")
}
func (chain *testChain) BlockHeight() uint32 {
	return atomic.LoadUint32(&chain.blockheight)
}
func (chain *testChain) Close() {
	panic("TODO")
}
func (chain testChain) HeaderHeight() uint32 {
	return 0
}
func (chain testChain) GetAppExecResult(hash util.Uint256) (*state.AppExecResult, error) {
	panic("TODO")
}
func (chain testChain) GetBlock(hash util.Uint256) (*block.Block, error) {
	panic("TODO")
}
func (chain testChain) GetContractState(hash util.Uint160) *state.Contract {
	panic("TODO")
}
func (chain testChain) GetHeaderHash(int) util.Uint256 {
	return util.Uint256{}
}
func (chain testChain) GetHeader(hash util.Uint256) (*block.Header, error) {
	panic("TODO")
}

func (chain testChain) GetAssetState(util.Uint256) *state.Asset {
	panic("TODO")
}
func (chain testChain) GetAccountState(util.Uint160) *state.Account {
	panic("TODO")
}
func (chain testChain) GetNEP5Metadata(util.Uint160) (*state.NEP5Metadata, error) {
	panic("TODO")
}
func (chain testChain) ForEachNEP5Transfer(util.Uint160, *state.NEP5Transfer, func() (bool, error)) error {
	panic("TODO")
}
func (chain testChain) GetNEP5Balances(util.Uint160) *state.NEP5Balances {
	panic("TODO")
}
func (chain testChain) GetValidators(...*transaction.Transaction) ([]*keys.PublicKey, error) {
	panic("TODO")
}
func (chain testChain) GetEnrollments() ([]*state.Validator, error) {
	panic("TODO")
}
func (chain testChain) ForEachTransfer(util.Uint160, *state.Transfer, func() (bool, error)) error {
	panic("TODO")
}
func (chain testChain) GetScriptHashesForVerifying(*transaction.Transaction) ([]util.Uint160, error) {
	panic("TODO")
}
func (chain testChain) GetStateProof(util.Uint256, []byte) ([][]byte, error) {
	panic("TODO")
}
func (chain testChain) GetStateRoot(height uint32) (*state.MPTRootState, error) {
	panic("TODO")
}
func (chain testChain) GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem {
	panic("TODO")
}
func (chain testChain) GetSystemFeeAmount(h util.Uint256) uint32 {
	panic("TODO")
}
func (chain testChain) GetTestVM(tx *transaction.Transaction) *vm.VM {
	panic("TODO")
}
func (chain testChain) GetStorageItems(hash util.Uint160) (map[string]*state.StorageItem, error) {
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

func (chain testChain) GetUnspentCoinState(util.Uint256) *state.UnspentCoin {
	panic("TODO")
}

func (chain testChain) GetMemPool() *mempool.Pool {
	panic("TODO")
}

func (chain testChain) IsLowPriority(util.Fixed8) bool {
	panic("TODO")
}

func (chain testChain) PoolTx(*transaction.Transaction) error {
	panic("TODO")
}
func (chain testChain) StateHeight() uint32 {
	panic("TODO")
}
func (chain testChain) SubscribeForBlocks(ch chan<- *block.Block) {
	panic("TODO")
}
func (chain testChain) SubscribeForExecutions(ch chan<- *state.AppExecResult) {
	panic("TODO")
}
func (chain testChain) SubscribeForNotifications(ch chan<- *state.NotificationEvent) {
	panic("TODO")
}
func (chain testChain) SubscribeForTransactions(ch chan<- *transaction.Transaction) {
	panic("TODO")
}

func (chain testChain) VerifyTx(*transaction.Transaction, *block.Block) error {
	panic("TODO")
}

func (chain testChain) UnsubscribeFromBlocks(ch chan<- *block.Block) {
	panic("TODO")
}
func (chain testChain) UnsubscribeFromExecutions(ch chan<- *state.AppExecResult) {
	panic("TODO")
}
func (chain testChain) UnsubscribeFromNotifications(ch chan<- *state.NotificationEvent) {
	panic("TODO")
}
func (chain testChain) UnsubscribeFromTransactions(ch chan<- *transaction.Transaction) {
	panic("TODO")
}

type testDiscovery struct{}

func (d testDiscovery) BackFill(addrs ...string)       {}
func (d testDiscovery) Close()                         {}
func (d testDiscovery) PoolCount() int                 { return 0 }
func (d testDiscovery) RegisterBadAddr(string)         {}
func (d testDiscovery) RegisterGoodAddr(string)        {}
func (d testDiscovery) RegisterConnectedAddr(string)   {}
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
	server         *Server
	version        *payload.Version
	lastBlockIndex uint32
	handshaked     bool
	t              *testing.T
	messageHandler func(t *testing.T, msg *Message)
	pingSent       int
}

func newLocalPeer(t *testing.T, s *Server) *localPeer {
	naddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	return &localPeer{
		t:              t,
		server:         s,
		netaddr:        *naddr,
		messageHandler: defaultMessageHandler,
	}
}

func (p *localPeer) RemoteAddr() net.Addr {
	return &p.netaddr
}
func (p *localPeer) PeerAddr() net.Addr {
	return &p.netaddr
}
func (p *localPeer) StartProtocol()       {}
func (p *localPeer) Disconnect(err error) {}

func (p *localPeer) EnqueueMessage(msg *Message) error {
	b, err := msg.Bytes()
	if err != nil {
		return err
	}
	return p.EnqueuePacket(b)
}
func (p *localPeer) EnqueuePacket(m []byte) error {
	return p.EnqueueHPPacket(m)
}
func (p *localPeer) EnqueueP2PMessage(msg *Message) error {
	return p.EnqueueMessage(msg)
}
func (p *localPeer) EnqueueP2PPacket(m []byte) error {
	return p.EnqueueHPPacket(m)
}
func (p *localPeer) EnqueueHPPacket(m []byte) error {
	msg := &Message{}
	r := io.NewBinReaderFromBuf(m)
	err := msg.Decode(r)
	if err == nil {
		p.messageHandler(p.t, msg)
	}
	return nil
}
func (p *localPeer) Version() *payload.Version {
	return p.version
}
func (p *localPeer) LastBlockIndex() uint32 {
	return p.lastBlockIndex
}
func (p *localPeer) HandleVersion(v *payload.Version) error {
	p.version = v
	return nil
}
func (p *localPeer) SendVersion() error {
	m := p.server.getVersionMsg()
	_ = p.EnqueueMessage(m)
	return nil
}
func (p *localPeer) SendVersionAck(m *Message) error {
	_ = p.EnqueueMessage(m)
	return nil
}
func (p *localPeer) HandleVersionAck() error {
	p.handshaked = true
	return nil
}
func (p *localPeer) SendPing(m *Message) error {
	p.pingSent++
	_ = p.EnqueueMessage(m)
	return nil
}
func (p *localPeer) HandlePong(pong *payload.Ping) error {
	p.lastBlockIndex = pong.LastBlockIndex
	p.pingSent--
	return nil
}

func (p *localPeer) Handshaked() bool {
	return p.handshaked
}

func newTestServer(t *testing.T) *Server {
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
		log:          zaptest.NewLogger(t),
	}

}
