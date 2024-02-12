package network

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/consensus"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type fakeConsensus struct {
	started  atomic.Bool
	stopped  atomic.Bool
	payloads []*payload.Extensible
	txlock   sync.Mutex
	txs      []*transaction.Transaction
}

var _ consensus.Service = (*fakeConsensus)(nil)

func (f *fakeConsensus) Name() string { return "fake" }
func (f *fakeConsensus) Start()       { f.started.Store(true) }
func (f *fakeConsensus) Shutdown()    { f.stopped.Store(true) }
func (f *fakeConsensus) OnPayload(p *payload.Extensible) error {
	f.payloads = append(f.payloads, p)
	return nil
}
func (f *fakeConsensus) OnTransaction(tx *transaction.Transaction) {
	f.txlock.Lock()
	defer f.txlock.Unlock()
	f.txs = append(f.txs, tx)
}
func (f *fakeConsensus) GetPayload(h util.Uint256) *payload.Extensible { panic("implement me") }

func TestNewServer(t *testing.T) {
	bc := &fakechain.FakeChain{Blockchain: config.Blockchain{
		ProtocolConfiguration: config.ProtocolConfiguration{
			P2PStateExchangeExtensions: true,
			StateRootInHeader:          true,
		}}}
	s, err := newServerFromConstructors(ServerConfig{}, bc, new(fakechain.FakeStateSync), nil, newFakeTransp, newTestDiscovery)
	require.Error(t, err)

	t.Run("set defaults", func(t *testing.T) {
		s = newTestServer(t, ServerConfig{MinPeers: -1})

		require.True(t, s.ID() != 0)
		require.Equal(t, defaultMinPeers, s.ServerConfig.MinPeers)
		require.Equal(t, defaultMaxPeers, s.ServerConfig.MaxPeers)
		require.Equal(t, defaultAttemptConnPeers, s.ServerConfig.AttemptConnPeers)
	})
	t.Run("don't defaults", func(t *testing.T) {
		cfg := ServerConfig{
			MinPeers:         1,
			MaxPeers:         2,
			AttemptConnPeers: 3,
		}
		s = newTestServer(t, cfg)

		require.True(t, s.ID() != 0)
		require.Equal(t, 1, s.ServerConfig.MinPeers)
		require.Equal(t, 2, s.ServerConfig.MaxPeers)
		require.Equal(t, 3, s.ServerConfig.AttemptConnPeers)
	})
}

func TestServerStartAndShutdown(t *testing.T) {
	t.Run("no consensus", func(t *testing.T) {
		s := newTestServer(t, ServerConfig{})

		go s.Start()
		p := newLocalPeer(t, s)
		s.register <- p
		require.Eventually(t, func() bool { return 1 == s.PeerCount() }, time.Second, time.Millisecond*10)

		assert.True(t, s.transports[0].(*fakeTransp).started.Load())
		assert.Nil(t, s.txCallback)

		s.Shutdown()

		require.True(t, s.transports[0].(*fakeTransp).closed.Load())
		err, ok := p.droppedWith.Load().(error)
		require.True(t, ok)
		require.ErrorIs(t, err, errServerShutdown)
	})
	t.Run("with consensus", func(t *testing.T) {
		s := newTestServer(t, ServerConfig{})
		cons := new(fakeConsensus)
		s.AddConsensusService(cons, cons.OnPayload, cons.OnTransaction)

		go s.Start()
		p := newLocalPeer(t, s)
		s.register <- p

		assert.True(t, s.services["fake"].(*fakeConsensus).started.Load())

		s.Shutdown()

		require.True(t, s.services["fake"].(*fakeConsensus).stopped.Load())
	})
}

func TestServerRegisterPeer(t *testing.T) {
	const peerCount = 3

	s := newTestServer(t, ServerConfig{MaxPeers: 2})
	defer s.Shutdown()

	ps := make([]*localPeer, peerCount)
	for i := range ps {
		ps[i] = newLocalPeer(t, s)
		ps[i].netaddr.Port = i + 1
		ps[i].version = &payload.Version{Nonce: uint32(i), UserAgent: []byte("fake")}
	}

	startWithCleanup(t, s)

	var wg sync.WaitGroup

	// Register and handshake peers
	for i, p := range ps {
		wg.Add(1)
		go func(peer *localPeer, index int) {
			defer wg.Done()
			s.register <- peer
			if index < 2 { // Simulate handshake for the first two peers only
				s.handshake <- peer
				t.Logf("Registered and handshaked peer: %v", peer.netaddr.Port)
			} else {
				s.handshake <- peer
				t.Logf("Attempted to register extra peer: %v", peer.netaddr.Port)
			}
		}(p, i)
	}

	wg.Wait()

	// Assert conditions after all peers have attempted registration and handshake
	require.Eventually(t, func() bool { return 2 == s.PeerCount() }, time.Second, time.Millisecond*10, "Expected 2 peers to be connected")
	require.GreaterOrEqual(t, len(s.discovery.UnconnectedPeers()), 1, "Expected unconnected peers due to MaxPeers limit")

	index := -1
	addrs := s.discovery.UnconnectedPeers()
	for _, addr := range addrs {
		for j := range ps {
			if ps[j].PeerAddr().String() == addr {
				index = j
				break
			}
		}
	}
	require.True(t, index >= 0)
	err, ok := ps[index].droppedWith.Load().(error)
	require.True(t, ok)
	require.ErrorIs(t, err, errMaxPeers)

	index = (index + 1) % peerCount
	s.unregister <- peerDrop{ps[index], errIdenticalID}
	require.Eventually(t, func() bool {
		bad := s.BadPeers()
		for i := range bad {
			if bad[i] == ps[index].PeerAddr().String() {
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond*50)
}

func TestGetBlocksByIndex(t *testing.T) {
	testGetBlocksByIndex(t, CMDGetBlockByIndex)
}

func testGetBlocksByIndex(t *testing.T, cmd CommandType) {
	s := newTestServer(t, ServerConfig{UserAgent: "/test/"})
	start := s.chain.BlockHeight()
	if cmd == CMDGetHeaders {
		start = s.chain.HeaderHeight()
		s.stateSync.(*fakechain.FakeStateSync).RequestHeaders.Store(true)
	}
	ps := make([]*localPeer, 10)
	expectsCmd := make([]CommandType, 10)
	expectedHeight := make([][]uint32, 10)
	for i := range ps {
		i := i
		ps[i] = newLocalPeer(t, s)
		ps[i].messageHandler = func(t *testing.T, msg *Message) {
			require.Equal(t, expectsCmd[i], msg.Command)
			if expectsCmd[i] == cmd {
				p, ok := msg.Payload.(*payload.GetBlockByIndex)
				require.True(t, ok)
				require.Contains(t, expectedHeight[i], p.IndexStart)
				expectsCmd[i] = CMDPong
			} else if expectsCmd[i] == CMDPong {
				expectsCmd[i] = cmd
			}
		}
		expectsCmd[i] = cmd
		expectedHeight[i] = []uint32{start + 1}
	}
	go s.transports[0].Accept()

	nonce := uint32(0)
	checkPingRespond := func(t *testing.T, peerIndex int, peerHeight uint32, hs ...uint32) {
		nonce++
		expectedHeight[peerIndex] = hs
		require.NoError(t, s.handlePing(ps[peerIndex], payload.NewPing(peerHeight, nonce)))
	}

	// Send all requests for all chunks.
	checkPingRespond(t, 0, 5000, 1)
	checkPingRespond(t, 1, 5000, 1+payload.MaxHashesCount)
	checkPingRespond(t, 2, 5000, 1+2*payload.MaxHashesCount)
	checkPingRespond(t, 3, 5000, 1+3*payload.MaxHashesCount)

	// Receive some blocks.
	s.chain.(*fakechain.FakeChain).Blockheight.Store(2123)

	// Minimum chunk has priority.
	checkPingRespond(t, 5, 5000, 2124)
	checkPingRespond(t, 6, 5000, 2624)
	// Request minimal height for peers behind.
	checkPingRespond(t, 7, 3100, 2124)
	checkPingRespond(t, 8, 5000, 3124)
	checkPingRespond(t, 9, 5000, 3624)
	// Request random height after that.
	checkPingRespond(t, 1, 5000, 2124, 2624, 3124, 3624)
	checkPingRespond(t, 2, 5000, 2124, 2624, 3124, 3624)
	checkPingRespond(t, 3, 5000, 2124, 2624, 3124, 3624)
}

func TestSendVersion(t *testing.T) {
	var (
		s = newTestServer(t, ServerConfig{UserAgent: "/test/"})
		p = newLocalPeer(t, s)
	)
	// we need to set listener at least to handle dynamic port correctly
	s.transports[0].Accept()
	p.messageHandler = func(t *testing.T, msg *Message) {
		// listener is already set, so Addresses(nil) gives us proper address with port
		_, prt := s.transports[0].HostPort()
		port, err := strconv.ParseUint(prt, 10, 16)
		assert.NoError(t, err)
		assert.Equal(t, CMDVersion, msg.Command)
		assert.IsType(t, msg.Payload, &payload.Version{})
		version := msg.Payload.(*payload.Version)
		assert.NotZero(t, version.Nonce)
		assert.Equal(t, 1, len(version.Capabilities))
		assert.ElementsMatch(t, []capability.Capability{
			{
				Type: capability.TCPServer,
				Data: &capability.Server{
					Port: uint16(port),
				},
			},
		}, version.Capabilities)
		assert.Equal(t, uint32(0), version.Version)
		assert.Equal(t, []byte("/test/"), version.UserAgent)
	}

	require.NoError(t, p.SendVersion())
}

// Server should reply with a verack after receiving a valid version.
func TestVerackAfterHandleVersionCmd(t *testing.T) {
	var (
		s = newTestServer(t, ServerConfig{})
		p = newLocalPeer(t, s)
	)
	na, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:3000")
	p.netaddr = *na

	// Should have a verack
	p.messageHandler = func(t *testing.T, msg *Message) {
		assert.Equal(t, CMDVerack, msg.Command)
	}
	capabilities := []capability.Capability{
		{
			Type: capability.FullNode,
			Data: &capability.Node{
				StartHeight: 0,
			},
		},
		{
			Type: capability.TCPServer,
			Data: &capability.Server{
				Port: 3000,
			},
		},
	}
	version := payload.NewVersion(0, 1337, "/NEO-GO/", capabilities)

	require.NoError(t, s.handleVersionCmd(p, version))
}

// Server should not reply with a verack after receiving a
// invalid version and disconnects the peer.
func TestServerNotSendsVerack(t *testing.T) {
	var (
		s  = newTestServer(t, ServerConfig{MaxPeers: 10, Net: 56753})
		p  = newLocalPeer(t, s)
		p2 = newLocalPeer(t, s)
	)
	s.id = 1
	finished := make(chan struct{})
	go func() {
		s.run()
		close(finished)
	}()
	t.Cleanup(func() {
		// close via quit as server was started via `run()`, not `Start()`
		close(s.quit)
		<-finished
	})

	na, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:3000")
	p.netaddr = *na
	p2.netaddr = *na
	s.register <- p

	capabilities := []capability.Capability{
		{
			Type: capability.FullNode,
			Data: &capability.Node{
				StartHeight: 0,
			},
		},
		{
			Type: capability.TCPServer,
			Data: &capability.Server{
				Port: 3000,
			},
		},
	}
	// identical id's
	version := payload.NewVersion(56753, 1, "/NEO-GO/", capabilities)
	err := s.handleVersionCmd(p, version)
	assert.NotNil(t, err)
	assert.Equal(t, errIdenticalID, err)

	// Different IDs, but also different magics
	version.Nonce = 2
	version.Magic = 56752
	err = s.handleVersionCmd(p, version)
	assert.NotNil(t, err)
	assert.Equal(t, errInvalidNetwork, err)

	// Different IDs and same network, make handshake pass.
	version.Magic = 56753
	require.NoError(t, s.handleVersionCmd(p, version))
	require.NoError(t, p.HandleVersionAck())
	require.Equal(t, true, p.Handshaked())

	// Second handshake from the same peer should fail.
	s.register <- p2
	err = s.handleVersionCmd(p2, version)
	assert.NotNil(t, err)
	require.Equal(t, errAlreadyConnected, err)
}

func (s *Server) testHandleMessage(t *testing.T, p Peer, cmd CommandType, pl payload.Payload) *Server {
	if p == nil {
		p = newLocalPeer(t, s)
		p.(*localPeer).handshaked = 1
	}
	msg := NewMessage(cmd, pl)
	require.NoError(t, s.handleMessage(p, msg))
	return s
}

func startTestServer(t *testing.T, protocolCfg ...func(*config.Blockchain)) *Server {
	var s *Server
	srvCfg := ServerConfig{UserAgent: "/test/"}
	if protocolCfg != nil {
		s = newTestServerWithCustomCfg(t, srvCfg, protocolCfg[0])
	} else {
		s = newTestServer(t, srvCfg)
	}
	startWithCleanup(t, s)
	return s
}

func startWithCleanup(t *testing.T, s *Server) {
	go s.Start()
	t.Cleanup(func() {
		s.Shutdown()
	})
}

func TestBlock(t *testing.T) {
	s := startTestServer(t)

	s.chain.(*fakechain.FakeChain).Blockheight.Store(12344)
	require.Equal(t, uint32(12344), s.chain.BlockHeight())

	b := block.New(false)
	b.Index = 12345
	s.testHandleMessage(t, nil, CMDBlock, b)
	require.Eventually(t, func() bool { return s.chain.BlockHeight() == 12345 }, 2*time.Second, time.Millisecond*500)
}

func TestConsensus(t *testing.T) {
	s := newTestServer(t, ServerConfig{})
	cons := new(fakeConsensus)
	s.AddConsensusService(cons, cons.OnPayload, cons.OnTransaction)
	startWithCleanup(t, s)

	s.chain.(*fakechain.FakeChain).Blockheight.Store(4)
	p := newLocalPeer(t, s)
	p.handshaked = 1
	s.register <- p
	require.Eventually(t, func() bool { return 1 == s.PeerCount() }, time.Second, time.Millisecond*10)

	newConsensusMessage := func(start, end uint32) *Message {
		pl := payload.NewExtensible()
		pl.Category = payload.ConsensusCategory
		pl.ValidBlockStart = start
		pl.ValidBlockEnd = end
		return NewMessage(CMDExtensible, pl)
	}

	s.chain.(*fakechain.FakeChain).VerifyWitnessF = func() (int64, error) { return 0, errors.New("invalid") }
	msg := newConsensusMessage(0, s.chain.BlockHeight()+1)
	require.Error(t, s.handleMessage(p, msg))

	s.chain.(*fakechain.FakeChain).VerifyWitnessF = func() (int64, error) { return 0, nil }
	require.NoError(t, s.handleMessage(p, msg))
	require.Contains(t, s.services["fake"].(*fakeConsensus).payloads, msg.Payload.(*payload.Extensible))

	t.Run("small ValidUntilBlockEnd", func(t *testing.T) {
		t.Run("current height", func(t *testing.T) {
			msg := newConsensusMessage(0, s.chain.BlockHeight())
			require.NoError(t, s.handleMessage(p, msg))
			require.NotContains(t, s.services["fake"].(*fakeConsensus).payloads, msg.Payload.(*payload.Extensible))
		})
		t.Run("invalid", func(t *testing.T) {
			msg := newConsensusMessage(0, s.chain.BlockHeight()-1)
			require.Error(t, s.handleMessage(p, msg))
		})
	})
	t.Run("big ValidUntiLBlockStart", func(t *testing.T) {
		msg := newConsensusMessage(s.chain.BlockHeight()+1, s.chain.BlockHeight()+2)
		require.Error(t, s.handleMessage(p, msg))
	})
}

func TestTransaction(t *testing.T) {
	s := newTestServer(t, ServerConfig{})
	cons := new(fakeConsensus)
	s.AddConsensusService(cons, cons.OnPayload, cons.OnTransaction)
	startWithCleanup(t, s)

	t.Run("good", func(t *testing.T) {
		tx := newDummyTx()
		s.RequestTx(tx.Hash())
		p := newLocalPeer(t, s)
		p.isFullNode = true
		p.messageHandler = func(t *testing.T, msg *Message) {
			if msg.Command == CMDInv {
				inv := msg.Payload.(*payload.Inventory)
				require.Equal(t, payload.TXType, inv.Type)
				require.Equal(t, []util.Uint256{tx.Hash()}, inv.Hashes)
			}
		}
		s.register <- p

		s.testHandleMessage(t, nil, CMDTX, tx)
		require.Eventually(t, func() bool {
			var fake = s.services["fake"].(*fakeConsensus)
			fake.txlock.Lock()
			defer fake.txlock.Unlock()
			for _, t := range fake.txs {
				if t == tx {
					return true
				}
			}
			return false
		}, 2*time.Second, time.Millisecond*500)
	})
	t.Run("bad", func(t *testing.T) {
		tx := newDummyTx()
		s.RequestTx(tx.Hash())
		s.chain.(*fakechain.FakeChain).PoolTxF = func(*transaction.Transaction) error { return core.ErrInsufficientFunds }
		s.testHandleMessage(t, nil, CMDTX, tx)
		require.Eventually(t, func() bool {
			var fake = s.services["fake"].(*fakeConsensus)
			fake.txlock.Lock()
			defer fake.txlock.Unlock()
			for _, t := range fake.txs {
				if t == tx {
					return true
				}
			}
			return false
		}, 2*time.Second, time.Millisecond*500)
	})
}

func (s *Server) testHandleGetData(t *testing.T, invType payload.InventoryType, hs, notFound []util.Uint256, found payload.Payload) {
	var recvResponse atomic.Bool
	var recvNotFound atomic.Bool

	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.messageHandler = func(t *testing.T, msg *Message) {
		switch msg.Command {
		case CMDTX, CMDBlock, CMDExtensible, CMDP2PNotaryRequest:
			require.Equal(t, found, msg.Payload)
			recvResponse.Store(true)
		case CMDNotFound:
			require.Equal(t, notFound, msg.Payload.(*payload.Inventory).Hashes)
			recvNotFound.Store(true)
		}
	}

	s.testHandleMessage(t, p, CMDGetData, payload.NewInventory(invType, hs))

	require.Eventually(t, func() bool { return recvResponse.Load() }, 2*time.Second, time.Millisecond)
	require.Eventually(t, func() bool { return recvNotFound.Load() }, 2*time.Second, time.Millisecond)
}

func TestGetData(t *testing.T) {
	s := startTestServer(t)
	defer s.Shutdown()
	s.chain.(*fakechain.FakeChain).UtilityTokenBalance = big.NewInt(1000000)

	t.Run("block", func(t *testing.T) {
		b := newDummyBlock(2, 0)
		hs := []util.Uint256{random.Uint256(), b.Hash(), random.Uint256()}
		s.chain.(*fakechain.FakeChain).PutBlock(b)
		notFound := []util.Uint256{hs[0], hs[2]}
		s.testHandleGetData(t, payload.BlockType, hs, notFound, b)
	})
	t.Run("transaction", func(t *testing.T) {
		tx := newDummyTx()
		hs := []util.Uint256{random.Uint256(), tx.Hash(), random.Uint256()}
		s.chain.(*fakechain.FakeChain).PutTx(tx)
		notFound := []util.Uint256{hs[0], hs[2]}
		s.testHandleGetData(t, payload.TXType, hs, notFound, tx)
	})
	t.Run("p2pNotaryRequest", func(t *testing.T) {
		mainTx := &transaction.Transaction{
			Attributes:      []transaction.Attribute{{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 1}}},
			Script:          []byte{0, 1, 2},
			ValidUntilBlock: 123,
			Signers:         []transaction.Signer{{Account: random.Uint160()}},
			Scripts:         []transaction.Witness{{InvocationScript: []byte{1, 2, 3}, VerificationScript: []byte{1, 2, 3}}},
		}
		mainTx.Size()
		mainTx.Hash()
		fallbackTx := &transaction.Transaction{
			Script:          []byte{1, 2, 3},
			ValidUntilBlock: 123,
			Attributes: []transaction.Attribute{
				{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}},
				{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: mainTx.Hash()}},
				{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 0}},
			},
			Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
			Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, make([]byte, keys.SignatureLen)...), VerificationScript: make([]byte, 0)}, {InvocationScript: []byte{}, VerificationScript: []byte{}}},
		}
		fallbackTx.Size()
		fallbackTx.Hash()
		r := &payload.P2PNotaryRequest{
			MainTransaction:     mainTx,
			FallbackTransaction: fallbackTx,
			Witness: transaction.Witness{
				InvocationScript:   []byte{1, 2, 3},
				VerificationScript: []byte{1, 2, 3},
			},
		}
		r.Hash()
		require.NoError(t, s.notaryRequestPool.Add(r.FallbackTransaction, s.chain, r))
		hs := []util.Uint256{random.Uint256(), r.FallbackTransaction.Hash(), random.Uint256()}
		notFound := []util.Uint256{hs[0], hs[2]}
		s.testHandleGetData(t, payload.P2PNotaryRequestType, hs, notFound, r)
	})
}

func initGetBlocksTest(t *testing.T) (*Server, []*block.Block) {
	s := startTestServer(t)

	var blocks []*block.Block
	for i := uint32(12); i <= 15; i++ {
		b := newDummyBlock(i, 3)
		s.chain.(*fakechain.FakeChain).PutBlock(b)
		blocks = append(blocks, b)
	}
	return s, blocks
}

func TestGetBlocks(t *testing.T) {
	s, blocks := initGetBlocksTest(t)

	expected := make([]util.Uint256, len(blocks))
	for i := range blocks {
		expected[i] = blocks[i].Hash()
	}
	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDInv {
			actual = msg.Payload.(*payload.Inventory).Hashes
		}
	}

	t.Run("2", func(t *testing.T) {
		s.testHandleMessage(t, p, CMDGetBlocks, &payload.GetBlocks{HashStart: expected[0], Count: 2})
		require.Equal(t, expected[1:3], actual)
	})
	t.Run("-1", func(t *testing.T) {
		s.testHandleMessage(t, p, CMDGetBlocks, &payload.GetBlocks{HashStart: expected[0], Count: -1})
		require.Equal(t, expected[1:], actual)
	})
	t.Run("invalid start", func(t *testing.T) {
		msg := NewMessage(CMDGetBlocks, &payload.GetBlocks{HashStart: util.Uint256{}, Count: -1})
		require.Error(t, s.handleMessage(p, msg))
	})
}

func TestGetBlockByIndex(t *testing.T) {
	s, blocks := initGetBlocksTest(t)

	var expected []*block.Block
	var actual []*block.Block
	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDBlock {
			actual = append(actual, msg.Payload.(*block.Block))
			if len(actual) == len(expected) {
				require.Equal(t, expected, actual)
			}
		}
	}

	t.Run("2", func(t *testing.T) {
		actual = nil
		expected = blocks[:2]
		s.testHandleMessage(t, p, CMDGetBlockByIndex, &payload.GetBlockByIndex{IndexStart: blocks[0].Index, Count: 2})
	})
	t.Run("-1", func(t *testing.T) {
		actual = nil
		expected = blocks
		s.testHandleMessage(t, p, CMDGetBlockByIndex, &payload.GetBlockByIndex{IndexStart: blocks[0].Index, Count: -1})
	})
	t.Run("-1, last header", func(t *testing.T) {
		s.chain.(*fakechain.FakeChain).PutHeader(newDummyBlock(16, 2))
		actual = nil
		expected = blocks
		s.testHandleMessage(t, p, CMDGetBlockByIndex, &payload.GetBlockByIndex{IndexStart: blocks[0].Index, Count: -1})
	})
}

func TestGetHeaders(t *testing.T) {
	s, blocks := initGetBlocksTest(t)

	expected := make([]*block.Header, len(blocks))
	for i := range blocks {
		expected[i] = &blocks[i].Header
	}

	var actual *payload.Headers
	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDHeaders {
			actual = msg.Payload.(*payload.Headers)
		}
	}

	t.Run("2", func(t *testing.T) {
		actual = nil
		s.testHandleMessage(t, p, CMDGetHeaders, &payload.GetBlockByIndex{IndexStart: blocks[0].Index, Count: 2})
		require.Equal(t, expected[:2], actual.Hdrs)
	})
	t.Run("more, than we have", func(t *testing.T) {
		actual = nil
		s.testHandleMessage(t, p, CMDGetHeaders, &payload.GetBlockByIndex{IndexStart: blocks[0].Index, Count: 10})
		require.Equal(t, expected, actual.Hdrs)
	})
	t.Run("-1", func(t *testing.T) {
		actual = nil
		s.testHandleMessage(t, p, CMDGetHeaders, &payload.GetBlockByIndex{IndexStart: blocks[0].Index, Count: -1})
		require.Equal(t, expected, actual.Hdrs)
	})
	t.Run("no headers", func(t *testing.T) {
		actual = nil
		s.testHandleMessage(t, p, CMDGetHeaders, &payload.GetBlockByIndex{IndexStart: 123, Count: -1})
		require.Nil(t, actual)
	})
	t.Run("distribute requests between peers", func(t *testing.T) {
		testGetBlocksByIndex(t, CMDGetHeaders)
	})
}

func TestInv(t *testing.T) {
	s := startTestServer(t)
	s.chain.(*fakechain.FakeChain).UtilityTokenBalance = big.NewInt(10000000)

	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDGetData {
			actual = msg.Payload.(*payload.Inventory).Hashes
		}
	}

	t.Run("blocks", func(t *testing.T) {
		b := newDummyBlock(10, 3)
		s.chain.(*fakechain.FakeChain).PutBlock(b)
		hs := []util.Uint256{random.Uint256(), b.Hash(), random.Uint256()}
		s.testHandleMessage(t, p, CMDInv, &payload.Inventory{
			Type:   payload.BlockType,
			Hashes: hs,
		})
		require.Equal(t, []util.Uint256{hs[0], hs[2]}, actual)
	})
	t.Run("transaction", func(t *testing.T) {
		tx := newDummyTx()
		require.NoError(t, s.chain.GetMemPool().Add(tx, s.chain))
		hs := []util.Uint256{random.Uint256(), tx.Hash(), random.Uint256()}
		s.testHandleMessage(t, p, CMDInv, &payload.Inventory{
			Type:   payload.TXType,
			Hashes: hs,
		})
		require.Equal(t, []util.Uint256{hs[0], hs[2]}, actual)
	})
	t.Run("extensible", func(t *testing.T) {
		ep := payload.NewExtensible()
		s.chain.(*fakechain.FakeChain).VerifyWitnessF = func() (int64, error) { return 0, nil }
		ep.ValidBlockEnd = s.chain.(*fakechain.FakeChain).BlockHeight() + 1
		ok, err := s.extensiblePool.Add(ep)
		require.NoError(t, err)
		require.True(t, ok)
		s.testHandleMessage(t, p, CMDInv, &payload.Inventory{
			Type:   payload.ExtensibleType,
			Hashes: []util.Uint256{ep.Hash()},
		})
	})
	t.Run("p2pNotaryRequest", func(t *testing.T) {
		fallbackTx := transaction.New(random.Bytes(100), 123)
		fallbackTx.Signers = []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}}
		fallbackTx.Size()
		fallbackTx.Hash()
		r := &payload.P2PNotaryRequest{
			MainTransaction:     newDummyTx(),
			FallbackTransaction: fallbackTx,
		}
		require.NoError(t, s.notaryRequestPool.Add(r.FallbackTransaction, s.chain, r))
		hs := []util.Uint256{random.Uint256(), r.FallbackTransaction.Hash(), random.Uint256()}
		s.testHandleMessage(t, p, CMDInv, &payload.Inventory{
			Type:   payload.P2PNotaryRequestType,
			Hashes: hs,
		})
		require.Equal(t, []util.Uint256{hs[0], hs[2]}, actual)
	})
}

func TestHandleGetMPTData(t *testing.T) {
	t.Run("P2PStateExchange extensions off", func(t *testing.T) {
		s := startTestServer(t)
		p := newLocalPeer(t, s)
		p.handshaked = 1
		msg := NewMessage(CMDGetMPTData, &payload.MPTInventory{
			Hashes: []util.Uint256{{1, 2, 3}},
		})
		require.Error(t, s.handleMessage(p, msg))
	})

	check := func(t *testing.T, s *Server) {
		var recvResponse atomic.Bool
		r1 := random.Uint256()
		r2 := random.Uint256()
		r3 := random.Uint256()
		node := []byte{1, 2, 3}
		s.stateSync.(*fakechain.FakeStateSync).TraverseFunc = func(root util.Uint256, process func(node mpt.Node, nodeBytes []byte) bool) error {
			if !(root.Equals(r1) || root.Equals(r2)) {
				t.Fatal("unexpected root")
			}
			require.False(t, process(mpt.NewHashNode(r3), node))
			return nil
		}
		found := &payload.MPTData{
			Nodes: [][]byte{node}, // no duplicates expected
		}
		p := newLocalPeer(t, s)
		p.handshaked = 1
		p.messageHandler = func(t *testing.T, msg *Message) {
			switch msg.Command {
			case CMDMPTData:
				require.Equal(t, found, msg.Payload)
				recvResponse.Store(true)
			}
		}
		hs := []util.Uint256{r1, r2}
		s.testHandleMessage(t, p, CMDGetMPTData, payload.NewMPTInventory(hs))

		require.Eventually(t, recvResponse.Load, time.Second, time.Millisecond)
	}
	t.Run("KeepOnlyLatestState on", func(t *testing.T) {
		s := startTestServer(t, func(c *config.Blockchain) {
			c.P2PStateExchangeExtensions = true
			c.Ledger.KeepOnlyLatestState = true
		})
		check(t, s)
	})

	t.Run("good", func(t *testing.T) {
		s := startTestServer(t, func(c *config.Blockchain) {
			c.P2PStateExchangeExtensions = true
		})
		check(t, s)
	})
}

func TestHandleMPTData(t *testing.T) {
	t.Run("P2PStateExchange extensions off", func(t *testing.T) {
		s := startTestServer(t)
		p := newLocalPeer(t, s)
		p.handshaked = 1
		msg := NewMessage(CMDMPTData, &payload.MPTData{
			Nodes: [][]byte{{1, 2, 3}},
		})
		require.Error(t, s.handleMessage(p, msg))
	})

	t.Run("good", func(t *testing.T) {
		expected := [][]byte{{1, 2, 3}, {2, 3, 4}}
		s := newTestServer(t, ServerConfig{UserAgent: "/test/"})
		s.config.P2PStateExchangeExtensions = true
		s.stateSync = &fakechain.FakeStateSync{
			AddMPTNodesFunc: func(nodes [][]byte) error {
				require.Equal(t, expected, nodes)
				return nil
			},
		}
		startWithCleanup(t, s)

		p := newLocalPeer(t, s)
		p.handshaked = 1
		msg := NewMessage(CMDMPTData, &payload.MPTData{
			Nodes: expected,
		})
		require.NoError(t, s.handleMessage(p, msg))
	})
}

func TestRequestMPTNodes(t *testing.T) {
	s := startTestServer(t)

	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDGetMPTData {
			actual = append(actual, msg.Payload.(*payload.MPTInventory).Hashes...)
		}
	}
	s.register <- p
	s.register <- p // ensure previous send was handled

	t.Run("no hashes, no message", func(t *testing.T) {
		actual = nil
		require.NoError(t, s.requestMPTNodes(p, nil))
		require.Nil(t, actual)
	})
	t.Run("good, small", func(t *testing.T) {
		actual = nil
		expected := []util.Uint256{random.Uint256(), random.Uint256()}
		require.NoError(t, s.requestMPTNodes(p, expected))
		require.Equal(t, expected, actual)
	})
	t.Run("good, exactly one chunk", func(t *testing.T) {
		actual = nil
		expected := make([]util.Uint256, payload.MaxMPTHashesCount)
		for i := range expected {
			expected[i] = random.Uint256()
		}
		require.NoError(t, s.requestMPTNodes(p, expected))
		require.Equal(t, expected, actual)
	})
	t.Run("good, too large chunk", func(t *testing.T) {
		actual = nil
		expected := make([]util.Uint256, payload.MaxMPTHashesCount+1)
		for i := range expected {
			expected[i] = random.Uint256()
		}
		require.NoError(t, s.requestMPTNodes(p, expected))
		require.Equal(t, expected[:payload.MaxMPTHashesCount], actual)
	})
}

func TestRequestTx(t *testing.T) {
	s := startTestServer(t)

	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDGetData {
			actual = append(actual, msg.Payload.(*payload.Inventory).Hashes...)
		}
	}
	s.register <- p
	s.register <- p // ensure previous send was handled

	t.Run("no hashes, no message", func(t *testing.T) {
		actual = nil
		s.RequestTx()
		require.Nil(t, actual)
	})
	t.Run("good, small", func(t *testing.T) {
		actual = nil
		expected := []util.Uint256{random.Uint256(), random.Uint256()}
		s.RequestTx(expected...)
		require.Equal(t, expected, actual)
	})
	t.Run("good, exactly one chunk", func(t *testing.T) {
		actual = nil
		expected := make([]util.Uint256, payload.MaxHashesCount)
		for i := range expected {
			expected[i] = random.Uint256()
		}
		s.RequestTx(expected...)
		require.Equal(t, expected, actual)
	})
	t.Run("good, multiple chunks", func(t *testing.T) {
		actual = nil
		expected := make([]util.Uint256, payload.MaxHashesCount*2+payload.MaxHashesCount/2)
		for i := range expected {
			expected[i] = random.Uint256()
		}
		s.RequestTx(expected...)
		require.Equal(t, expected, actual)
	})
}

func TestAddrs(t *testing.T) {
	s := startTestServer(t)

	ips := make([][16]byte, 4)
	copy(ips[0][:], net.IPv4(1, 2, 3, 4))
	copy(ips[1][:], net.IPv4(7, 8, 9, 0))
	for i := range ips[2] {
		ips[2][i] = byte(i)
	}

	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.getAddrSent = 1
	pl := &payload.AddressList{
		Addrs: []*payload.AddressAndTime{
			{
				IP: ips[0],
				Capabilities: capability.Capabilities{{
					Type: capability.TCPServer,
					Data: &capability.Server{Port: 12},
				}},
			},
			{
				IP:           ips[1],
				Capabilities: capability.Capabilities{},
			},
			{
				IP: ips[2],
				Capabilities: capability.Capabilities{{
					Type: capability.TCPServer,
					Data: &capability.Server{Port: 42},
				}},
			},
		},
	}
	s.testHandleMessage(t, p, CMDAddr, pl)

	addrs := s.discovery.(*testDiscovery).backfill
	require.Equal(t, 2, len(addrs))
	require.Equal(t, "1.2.3.4:12", addrs[0])
	require.Equal(t, net.JoinHostPort(net.IP(ips[2][:]).String(), "42"), addrs[1])

	t.Run("CMDAddr not requested", func(t *testing.T) {
		msg := NewMessage(CMDAddr, pl)
		require.Error(t, s.handleMessage(p, msg))
	})
}

type feerStub struct {
	blockHeight uint32
}

func (f feerStub) FeePerByte() int64                            { return 1 }
func (f feerStub) GetUtilityTokenBalance(util.Uint160) *big.Int { return big.NewInt(100000000) }
func (f feerStub) BlockHeight() uint32                          { return f.blockHeight }
func (f feerStub) GetBaseExecFee() int64                        { return interop.DefaultBaseExecFee }

func TestMemPool(t *testing.T) {
	s := startTestServer(t)

	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = 1
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDInv {
			actual = append(actual, msg.Payload.(*payload.Inventory).Hashes...)
		}
	}

	bc := s.chain.(*fakechain.FakeChain)
	expected := make([]util.Uint256, 4)
	for i := range expected {
		tx := newDummyTx()
		require.NoError(t, bc.Pool.Add(tx, &feerStub{blockHeight: 10}))
		expected[i] = tx.Hash()
	}

	s.testHandleMessage(t, p, CMDMempool, payload.NullPayload{})
	require.ElementsMatch(t, expected, actual)
}

func TestVerifyNotaryRequest(t *testing.T) {
	bc := fakechain.NewFakeChain()
	bc.MaxVerificationGAS = 10
	bc.NotaryContractScriptHash = util.Uint160{1, 2, 3}
	s, err := newServerFromConstructors(ServerConfig{Addresses: []config.AnnounceableAddress{{Address: ":0"}}}, bc, new(fakechain.FakeStateSync), zaptest.NewLogger(t), newFakeTransp, newTestDiscovery)
	require.NoError(t, err)
	newNotaryRequest := func() *payload.P2PNotaryRequest {
		return &payload.P2PNotaryRequest{
			MainTransaction: &transaction.Transaction{
				Script:  []byte{0, 1, 2},
				Signers: []transaction.Signer{{Account: random.Uint160()}},
			},
			FallbackTransaction: &transaction.Transaction{
				ValidUntilBlock: 321,
				Signers:         []transaction.Signer{{Account: bc.NotaryContractScriptHash}, {Account: random.Uint160()}},
			},
			Witness: transaction.Witness{},
		}
	}

	t.Run("bad payload witness", func(t *testing.T) {
		bc.VerifyWitnessF = func() (int64, error) { return 0, errors.New("bad witness") }
		require.Error(t, s.verifyNotaryRequest(nil, newNotaryRequest()))
	})

	t.Run("bad fallback sender", func(t *testing.T) {
		bc.VerifyWitnessF = func() (int64, error) { return 0, nil }
		r := newNotaryRequest()
		r.FallbackTransaction.Signers[0] = transaction.Signer{Account: util.Uint160{7, 8, 9}}
		require.Error(t, s.verifyNotaryRequest(nil, r))
	})

	t.Run("bad main sender", func(t *testing.T) {
		bc.VerifyWitnessF = func() (int64, error) { return 0, nil }
		r := newNotaryRequest()
		r.MainTransaction.Signers[0] = transaction.Signer{Account: bc.NotaryContractScriptHash}
		require.Error(t, s.verifyNotaryRequest(nil, r))
	})

	t.Run("expired deposit", func(t *testing.T) {
		r := newNotaryRequest()
		bc.NotaryDepositExpiration = r.FallbackTransaction.ValidUntilBlock
		require.Error(t, s.verifyNotaryRequest(nil, r))
	})

	t.Run("good", func(t *testing.T) {
		r := newNotaryRequest()
		bc.NotaryDepositExpiration = r.FallbackTransaction.ValidUntilBlock + 1
		require.NoError(t, s.verifyNotaryRequest(nil, r))
	})
}

func TestTryInitStateSync(t *testing.T) {
	t.Run("module inactive", func(t *testing.T) {
		s := startTestServer(t)
		s.tryInitStateSync()
	})

	t.Run("module already initialized", func(t *testing.T) {
		s := startTestServer(t)
		ss := &fakechain.FakeStateSync{}
		ss.IsActiveFlag.Store(true)
		ss.IsInitializedFlag.Store(true)
		s.stateSync = ss
		s.tryInitStateSync()
	})

	t.Run("good", func(t *testing.T) {
		s := startTestServer(t)
		for _, h := range []uint32{10, 8, 7, 4, 11, 4} {
			p := newLocalPeer(t, s)
			p.handshaked = 1
			p.lastBlockIndex = h
			s.register <- p
		}
		p := newLocalPeer(t, s)
		p.handshaked = 0 // one disconnected peer to check it won't be taken into attention
		p.lastBlockIndex = 5
		s.register <- p
		require.Eventually(t, func() bool { return 7 == s.PeerCount() }, time.Second, time.Millisecond*10)

		var expectedH uint32 = 8 // median peer
		ss := &fakechain.FakeStateSync{InitFunc: func(h uint32) error {
			if h != expectedH {
				return fmt.Errorf("invalid height: expected %d, got %d", expectedH, h)
			}
			return nil
		}}
		ss.IsActiveFlag.Store(true)
		s.stateSync = ss
		s.tryInitStateSync()
	})
}

func TestServer_Port(t *testing.T) {
	s := newTestServer(t, ServerConfig{
		Addresses: []config.AnnounceableAddress{
			{Address: "1.2.3.4:10"},                           // some random address
			{Address: ":1"},                                   // listen all IPs
			{Address: "127.0.0.1:2"},                          // address without announced port
			{Address: "123.123.0.123:3", AnnouncedPort: 123}}, // address with announced port
	})

	// Default addr => first port available
	actual, err := s.Port(nil)
	require.NoError(t, err)
	require.Equal(t, uint16(10), actual)

	// Specified address with direct match => port of matched address
	actual, err = s.Port(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 123})
	require.NoError(t, err)
	require.Equal(t, uint16(2), actual)

	// No address match => 0.0.0.0's port
	actual, err = s.Port(&net.TCPAddr{IP: net.IPv4(5, 6, 7, 8), Port: 123})
	require.NoError(t, err)
	require.Equal(t, uint16(1), actual)

	// Specified address with match on announceable address => announced port
	actual, err = s.Port(&net.TCPAddr{IP: net.IPv4(123, 123, 0, 123), Port: 123})
	require.NoError(t, err)
	require.Equal(t, uint16(123), actual)
}
