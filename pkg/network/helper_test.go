package network

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer/services"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type testChain struct {
	config.ProtocolConfiguration
	*mempool.Pool
	blocksCh                 []chan<- *block.Block
	blockheight              uint32
	poolTx                   func(*transaction.Transaction) error
	poolTxWithData           func(*transaction.Transaction, interface{}, *mempool.Pool) error
	blocks                   map[util.Uint256]*block.Block
	hdrHashes                map[uint32]util.Uint256
	txs                      map[util.Uint256]*transaction.Transaction
	verifyWitnessF           func() error
	maxVerificationGAS       int64
	notaryContractScriptHash util.Uint160
	notaryDepositExpiration  uint32
	postBlock                []func(blockchainer.Blockchainer, *mempool.Pool, *block.Block)
	utilityTokenBalance      *big.Int
}

func newTestChain() *testChain {
	return &testChain{
		Pool:                  mempool.New(10, 0, false),
		poolTx:                func(*transaction.Transaction) error { return nil },
		poolTxWithData:        func(*transaction.Transaction, interface{}, *mempool.Pool) error { return nil },
		blocks:                make(map[util.Uint256]*block.Block),
		hdrHashes:             make(map[uint32]util.Uint256),
		txs:                   make(map[util.Uint256]*transaction.Transaction),
		ProtocolConfiguration: config.ProtocolConfiguration{P2PNotaryRequestPayloadPoolSize: 10},
	}
}

func (chain *testChain) putBlock(b *block.Block) {
	chain.blocks[b.Hash()] = b
	chain.hdrHashes[b.Index] = b.Hash()
	atomic.StoreUint32(&chain.blockheight, b.Index)
}
func (chain *testChain) putHeader(b *block.Block) {
	chain.hdrHashes[b.Index] = b.Hash()
}

func (chain *testChain) putTx(tx *transaction.Transaction) {
	chain.txs[tx.Hash()] = tx
}

func (chain *testChain) ApplyPolicyToTxSet([]*transaction.Transaction) []*transaction.Transaction {
	panic("TODO")
}

func (chain *testChain) IsTxStillRelevant(t *transaction.Transaction, txpool *mempool.Pool, isPartialTx bool) bool {
	panic("TODO")
}
func (*testChain) IsExtensibleAllowed(uint160 util.Uint160) bool {
	return true
}

func (chain *testChain) GetNotaryDepositExpiration(acc util.Uint160) uint32 {
	if chain.notaryDepositExpiration != 0 {
		return chain.notaryDepositExpiration
	}
	panic("TODO")
}

func (chain *testChain) GetNotaryContractScriptHash() util.Uint160 {
	if !chain.notaryContractScriptHash.Equals(util.Uint160{}) {
		return chain.notaryContractScriptHash
	}
	panic("TODO")
}

func (chain *testChain) GetNotaryBalance(acc util.Uint160) *big.Int {
	panic("TODO")
}

func (chain *testChain) GetPolicer() blockchainer.Policer {
	return chain
}
func (chain *testChain) GetBaseExecFee() int64 {
	return interop.DefaultBaseExecFee
}
func (chain *testChain) GetStoragePrice() int64 {
	return native.StoragePrice
}
func (chain *testChain) GetMaxVerificationGAS() int64 {
	if chain.maxVerificationGAS != 0 {
		return chain.maxVerificationGAS
	}
	panic("TODO")
}

func (chain *testChain) PoolTxWithData(t *transaction.Transaction, data interface{}, mp *mempool.Pool, feer mempool.Feer, verificationFunction func(bc blockchainer.Blockchainer, t *transaction.Transaction, data interface{}) error) error {
	return chain.poolTxWithData(t, data, mp)
}

func (chain *testChain) RegisterPostBlock(f func(blockchainer.Blockchainer, *mempool.Pool, *block.Block)) {
	chain.postBlock = append(chain.postBlock, f)
}

func (chain *testChain) GetConfig() config.ProtocolConfiguration {
	return chain.ProtocolConfiguration
}
func (chain *testChain) CalculateClaimable(util.Uint160, uint32) (*big.Int, error) {
	panic("TODO")
}

func (chain *testChain) FeePerByte() int64 {
	panic("TODO")
}

func (chain *testChain) P2PSigExtensionsEnabled() bool {
	return true
}

func (chain *testChain) GetMaxBlockSystemFee() int64 {
	panic("TODO")
}

func (chain *testChain) GetMaxBlockSize() uint32 {
	panic("TODO")
}

func (chain *testChain) AddHeaders(...*block.Header) error {
	panic("TODO")
}
func (chain *testChain) AddBlock(block *block.Block) error {
	if block.Index == atomic.LoadUint32(&chain.blockheight)+1 {
		chain.putBlock(block)
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
func (chain *testChain) HeaderHeight() uint32 {
	return atomic.LoadUint32(&chain.blockheight)
}
func (chain *testChain) GetAppExecResults(hash util.Uint256, trig trigger.Type) ([]state.AppExecResult, error) {
	panic("TODO")
}
func (chain *testChain) GetBlock(hash util.Uint256) (*block.Block, error) {
	if b, ok := chain.blocks[hash]; ok {
		return b, nil
	}
	return nil, errors.New("not found")
}
func (chain *testChain) GetCommittee() (keys.PublicKeys, error) {
	panic("TODO")
}
func (chain *testChain) GetContractState(hash util.Uint160) *state.Contract {
	panic("TODO")
}
func (chain *testChain) GetContractScriptHash(id int32) (util.Uint160, error) {
	panic("TODO")
}
func (chain *testChain) GetNativeContractScriptHash(name string) (util.Uint160, error) {
	panic("TODO")
}
func (chain *testChain) GetHeaderHash(n int) util.Uint256 {
	return chain.hdrHashes[uint32(n)]
}
func (chain *testChain) GetHeader(hash util.Uint256) (*block.Header, error) {
	b, err := chain.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	return b.Header(), nil
}

func (chain *testChain) GetNextBlockValidators() ([]*keys.PublicKey, error) {
	panic("TODO")
}
func (chain *testChain) ForEachNEP17Transfer(util.Uint160, func(*state.NEP17Transfer) (bool, error)) error {
	panic("TODO")
}
func (chain *testChain) GetNEP17Balances(util.Uint160) *state.NEP17Balances {
	panic("TODO")
}
func (chain *testChain) GetValidators() ([]*keys.PublicKey, error) {
	panic("TODO")
}
func (chain *testChain) GetStandByCommittee() keys.PublicKeys {
	panic("TODO")
}
func (chain *testChain) GetStandByValidators() keys.PublicKeys {
	panic("TODO")
}
func (chain *testChain) GetEnrollments() ([]state.Validator, error) {
	panic("TODO")
}
func (chain *testChain) GetStateProof(util.Uint256, []byte) ([][]byte, error) {
	panic("TODO")
}
func (chain *testChain) GetStateRoot(height uint32) (*state.MPTRootState, error) {
	panic("TODO")
}
func (chain *testChain) GetStorageItem(id int32, key []byte) *state.StorageItem {
	panic("TODO")
}
func (chain *testChain) GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) *vm.VM {
	panic("TODO")
}
func (chain *testChain) GetStorageItems(id int32) (map[string]*state.StorageItem, error) {
	panic("TODO")
}
func (chain *testChain) CurrentHeaderHash() util.Uint256 {
	return util.Uint256{}
}
func (chain *testChain) CurrentBlockHash() util.Uint256 {
	return util.Uint256{}
}
func (chain *testChain) HasBlock(h util.Uint256) bool {
	_, ok := chain.blocks[h]
	return ok
}
func (chain *testChain) HasTransaction(h util.Uint256) bool {
	_, ok := chain.txs[h]
	return ok
}
func (chain *testChain) GetTransaction(h util.Uint256) (*transaction.Transaction, uint32, error) {
	if tx, ok := chain.txs[h]; ok {
		return tx, 1, nil
	}
	return nil, 0, errors.New("not found")
}

func (chain *testChain) GetMemPool() *mempool.Pool {
	return chain.Pool
}

func (chain *testChain) GetGoverningTokenBalance(acc util.Uint160) (*big.Int, uint32) {
	panic("TODO")
}

func (chain *testChain) GetUtilityTokenBalance(uint160 util.Uint160) *big.Int {
	if chain.utilityTokenBalance != nil {
		return chain.utilityTokenBalance
	}
	panic("TODO")
}
func (chain testChain) ManagementContractHash() util.Uint160 {
	panic("TODO")
}

func (chain *testChain) PoolTx(tx *transaction.Transaction, _ ...*mempool.Pool) error {
	return chain.poolTx(tx)
}
func (chain testChain) SetOracle(services.Oracle) {
	panic("TODO")
}
func (chain *testChain) SetNotary(notary services.Notary) {
	panic("TODO")
}
func (chain *testChain) SubscribeForBlocks(ch chan<- *block.Block) {
	chain.blocksCh = append(chain.blocksCh, ch)
}
func (chain *testChain) SubscribeForExecutions(ch chan<- *state.AppExecResult) {
	panic("TODO")
}
func (chain *testChain) SubscribeForNotifications(ch chan<- *state.NotificationEvent) {
	panic("TODO")
}
func (chain *testChain) SubscribeForTransactions(ch chan<- *transaction.Transaction) {
	panic("TODO")
}

func (chain *testChain) VerifyTx(*transaction.Transaction) error {
	panic("TODO")
}
func (chain *testChain) VerifyWitness(util.Uint160, crypto.Verifiable, *transaction.Witness, int64) error {
	if chain.verifyWitnessF != nil {
		return chain.verifyWitnessF()
	}
	panic("TODO")
}

func (chain *testChain) UnsubscribeFromBlocks(ch chan<- *block.Block) {
	for i, c := range chain.blocksCh {
		if c == ch {
			if i < len(chain.blocksCh) {
				copy(chain.blocksCh[i:], chain.blocksCh[i+1:])
			}
			chain.blocksCh = chain.blocksCh[:len(chain.blocksCh)]
		}
	}
}
func (chain *testChain) UnsubscribeFromExecutions(ch chan<- *state.AppExecResult) {
	panic("TODO")
}
func (chain *testChain) UnsubscribeFromNotifications(ch chan<- *state.NotificationEvent) {
	panic("TODO")
}
func (chain *testChain) UnsubscribeFromTransactions(ch chan<- *transaction.Transaction) {
	panic("TODO")
}

type testDiscovery struct {
	sync.Mutex
	bad          []string
	good         []string
	connected    []string
	unregistered []string
	backfill     []string
}

func newTestDiscovery([]string, time.Duration, Transporter) Discoverer { return new(testDiscovery) }

func (d *testDiscovery) BackFill(addrs ...string) {
	d.Lock()
	defer d.Unlock()
	d.backfill = append(d.backfill, addrs...)
}
func (d *testDiscovery) Close()         {}
func (d *testDiscovery) PoolCount() int { return 0 }
func (d *testDiscovery) RegisterBadAddr(addr string) {
	d.Lock()
	defer d.Unlock()
	d.bad = append(d.bad, addr)
}
func (d *testDiscovery) RegisterGoodAddr(string, capability.Capabilities) {}
func (d *testDiscovery) RegisterConnectedAddr(addr string) {
	d.Lock()
	defer d.Unlock()
	d.connected = append(d.connected, addr)
}
func (d *testDiscovery) UnregisterConnectedAddr(addr string) {
	d.Lock()
	defer d.Unlock()
	d.unregistered = append(d.unregistered, addr)
}
func (d *testDiscovery) UnconnectedPeers() []string {
	d.Lock()
	defer d.Unlock()
	return d.unregistered
}
func (d *testDiscovery) RequestRemote(n int) {}
func (d *testDiscovery) BadPeers() []string {
	d.Lock()
	defer d.Unlock()
	return d.bad
}
func (d *testDiscovery) GoodPeers() []AddressWithCapabilities { return []AddressWithCapabilities{} }

var defaultMessageHandler = func(t *testing.T, msg *Message) {}

type localPeer struct {
	netaddr        net.TCPAddr
	server         *Server
	version        *payload.Version
	lastBlockIndex uint32
	handshaked     bool
	isFullNode     bool
	t              *testing.T
	messageHandler func(t *testing.T, msg *Message)
	pingSent       int
	getAddrSent    int
	droppedWith    atomic.Value
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
func (p *localPeer) StartProtocol() {}
func (p *localPeer) Disconnect(err error) {
	if p.droppedWith.Load() == nil {
		p.droppedWith.Store(err)
	}
	fmt.Println("peer dropped:", err)
	p.server.unregister <- peerDrop{p, err}
}

func (p *localPeer) EnqueueMessage(msg *Message) error {
	b, err := msg.Bytes()
	if err != nil {
		return err
	}
	return p.EnqueuePacket(true, b)
}
func (p *localPeer) EnqueuePacket(block bool, m []byte) error {
	return p.EnqueueHPPacket(block, m)
}
func (p *localPeer) EnqueueP2PMessage(msg *Message) error {
	return p.EnqueueMessage(msg)
}
func (p *localPeer) EnqueueP2PPacket(m []byte) error {
	return p.EnqueueHPPacket(true, m)
}
func (p *localPeer) EnqueueHPPacket(_ bool, m []byte) error {
	msg := &Message{Network: netmode.UnitTestNet}
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
	m, err := p.server.getVersionMsg()
	if err != nil {
		return err
	}
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
func (p *localPeer) HandlePing(ping *payload.Ping) error {
	p.lastBlockIndex = ping.LastBlockIndex
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

func (p *localPeer) IsFullNode() bool {
	return p.isFullNode
}

func (p *localPeer) AddGetAddrSent() {
	p.getAddrSent++
}
func (p *localPeer) CanProcessAddr() bool {
	p.getAddrSent--
	return p.getAddrSent >= 0
}

func newTestServer(t *testing.T, serverConfig ServerConfig) *Server {
	s, err := newServerFromConstructors(serverConfig, newTestChain(), zaptest.NewLogger(t),
		newFakeTransp, newFakeConsensus, newTestDiscovery)
	require.NoError(t, err)
	return s
}
