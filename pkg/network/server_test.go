package network

import (
	"errors"
	"math/big"
	"net"
	"strconv"
	atomic2 "sync/atomic"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/consensus"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap/zaptest"
)

type fakeConsensus struct {
	started  atomic.Bool
	stopped  atomic.Bool
	payloads []*payload.Extensible
	txs      []*transaction.Transaction
}

var _ consensus.Service = (*fakeConsensus)(nil)

func newFakeConsensus(c consensus.Config) (consensus.Service, error) {
	return new(fakeConsensus), nil
}
func (f *fakeConsensus) Start()                                        { f.started.Store(true) }
func (f *fakeConsensus) Shutdown()                                     { f.stopped.Store(true) }
func (f *fakeConsensus) OnPayload(p *payload.Extensible)               { f.payloads = append(f.payloads, p) }
func (f *fakeConsensus) OnTransaction(tx *transaction.Transaction)     { f.txs = append(f.txs, tx) }
func (f *fakeConsensus) GetPayload(h util.Uint256) *payload.Extensible { panic("implement me") }

func TestNewServer(t *testing.T) {
	bc := &fakechain.FakeChain{ProtocolConfiguration: config.ProtocolConfiguration{
		P2PStateExchangeExtensions: true,
		StateRootInHeader:          true,
	}}
	s, err := newServerFromConstructors(ServerConfig{}, bc, nil, newFakeTransp, newFakeConsensus, newTestDiscovery)
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
	t.Run("consensus error is not dropped", func(t *testing.T) {
		errConsensus := errors.New("can't create consensus")
		_, err = newServerFromConstructors(ServerConfig{MinPeers: -1}, bc, zaptest.NewLogger(t), newFakeTransp,
			func(consensus.Config) (consensus.Service, error) { return nil, errConsensus },
			newTestDiscovery)
		require.True(t, errors.Is(err, errConsensus), "got: %#v", err)
	})
	t.Run("StateExchangeExtensions enabled with StateRootInHeader off", func(t *testing.T) {
		bc.ProtocolConfiguration = config.ProtocolConfiguration{
			P2PStateExchangeExtensions: true,
			StateRootInHeader:          false,
		}
		_, err := newServerFromConstructors(ServerConfig{}, bc, zaptest.NewLogger(t), newFakeTransp, newFakeConsensus, newTestDiscovery)
		require.Error(t, err)
	})
}

func startWithChannel(s *Server) chan error {
	ch := make(chan error)
	go func() {
		s.Start(ch)
		close(ch)
	}()
	return ch
}

func TestServerStartAndShutdown(t *testing.T) {
	t.Run("no consensus", func(t *testing.T) {
		s := newTestServer(t, ServerConfig{})

		ch := startWithChannel(s)
		p := newLocalPeer(t, s)
		s.register <- p
		require.Eventually(t, func() bool { return 1 == s.PeerCount() }, time.Second, time.Millisecond*10)

		assert.True(t, s.transport.(*fakeTransp).started.Load())
		assert.False(t, s.consensus.(*fakeConsensus).started.Load())

		s.Shutdown()
		<-ch

		require.True(t, s.transport.(*fakeTransp).closed.Load())
		require.True(t, s.consensus.(*fakeConsensus).stopped.Load())
		err, ok := p.droppedWith.Load().(error)
		require.True(t, ok)
		require.True(t, errors.Is(err, errServerShutdown))
	})
	t.Run("with consensus", func(t *testing.T) {
		s := newTestServer(t, ServerConfig{Wallet: new(config.Wallet)})

		ch := startWithChannel(s)
		p := newLocalPeer(t, s)
		s.register <- p

		assert.True(t, s.consensus.(*fakeConsensus).started.Load())

		s.Shutdown()
		<-ch

		require.True(t, s.consensus.(*fakeConsensus).stopped.Load())
	})
}

func TestServerRegisterPeer(t *testing.T) {
	const peerCount = 3

	s := newTestServer(t, ServerConfig{MaxPeers: 2})
	ps := make([]*localPeer, peerCount)
	for i := range ps {
		ps[i] = newLocalPeer(t, s)
		ps[i].netaddr.Port = i + 1
	}

	ch := startWithChannel(s)
	t.Cleanup(func() {
		s.Shutdown()
		<-ch
	})

	s.register <- ps[0]
	require.Eventually(t, func() bool { return 1 == s.PeerCount() }, time.Second, time.Millisecond*10)

	s.register <- ps[1]
	require.Eventually(t, func() bool { return 2 == s.PeerCount() }, time.Second, time.Millisecond*10)

	require.Equal(t, 0, len(s.discovery.UnconnectedPeers()))
	s.register <- ps[2]
	require.Eventually(t, func() bool { return len(s.discovery.UnconnectedPeers()) > 0 }, time.Second, time.Millisecond*100)

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
	require.True(t, errors.Is(err, errMaxPeers))

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
	s := newTestServer(t, ServerConfig{Port: 0, UserAgent: "/test/"})
	ps := make([]*localPeer, 10)
	expectsCmd := make([]CommandType, 10)
	expectedHeight := make([][]uint32, 10)
	start := s.chain.BlockHeight()
	for i := range ps {
		i := i
		ps[i] = newLocalPeer(t, s)
		ps[i].messageHandler = func(t *testing.T, msg *Message) {
			require.Equal(t, expectsCmd[i], msg.Command)
			if expectsCmd[i] == CMDGetBlockByIndex {
				p, ok := msg.Payload.(*payload.GetBlockByIndex)
				require.True(t, ok)
				require.Contains(t, expectedHeight[i], p.IndexStart)
				expectsCmd[i] = CMDPong
			} else if expectsCmd[i] == CMDPong {
				expectsCmd[i] = CMDGetBlockByIndex
			}
		}
		expectsCmd[i] = CMDGetBlockByIndex
		expectedHeight[i] = []uint32{start + 1}
	}
	go s.transport.Accept()

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
	s.chain.(*fakechain.FakeChain).Blockheight = 2123

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
		s = newTestServer(t, ServerConfig{Port: 0, UserAgent: "/test/"})
		p = newLocalPeer(t, s)
	)
	// we need to set listener at least to handle dynamic port correctly
	s.transport.Accept()
	p.messageHandler = func(t *testing.T, msg *Message) {
		// listener is already set, so Address() gives us proper address with port
		_, p, err := net.SplitHostPort(s.transport.Address())
		assert.NoError(t, err)
		port, err := strconv.ParseUint(p, 10, 16)
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
		p.(*localPeer).handshaked = true
	}
	msg := NewMessage(cmd, pl)
	require.NoError(t, s.handleMessage(p, msg))
	return s
}

func startTestServer(t *testing.T) *Server {
	s := newTestServer(t, ServerConfig{Port: 0, UserAgent: "/test/"})
	ch := startWithChannel(s)
	t.Cleanup(func() {
		s.Shutdown()
		<-ch
	})
	return s
}

func TestBlock(t *testing.T) {
	s := startTestServer(t)

	atomic2.StoreUint32(&s.chain.(*fakechain.FakeChain).Blockheight, 12344)
	require.Equal(t, uint32(12344), s.chain.BlockHeight())

	b := block.New(false)
	b.Index = 12345
	s.testHandleMessage(t, nil, CMDBlock, b)
	require.Eventually(t, func() bool { return s.chain.BlockHeight() == 12345 }, time.Second, time.Millisecond*500)
}

func TestConsensus(t *testing.T) {
	s := startTestServer(t)

	atomic2.StoreUint32(&s.chain.(*fakechain.FakeChain).Blockheight, 4)
	p := newLocalPeer(t, s)
	p.handshaked = true
	s.register <- p
	require.Eventually(t, func() bool { return 1 == s.PeerCount() }, time.Second, time.Millisecond*10)

	newConsensusMessage := func(start, end uint32) *Message {
		pl := payload.NewExtensible()
		pl.Category = consensus.Category
		pl.ValidBlockStart = start
		pl.ValidBlockEnd = end
		return NewMessage(CMDExtensible, pl)
	}

	s.chain.(*fakechain.FakeChain).VerifyWitnessF = func() error { return errors.New("invalid") }
	msg := newConsensusMessage(0, s.chain.BlockHeight()+1)
	require.Error(t, s.handleMessage(p, msg))

	s.chain.(*fakechain.FakeChain).VerifyWitnessF = func() error { return nil }
	require.NoError(t, s.handleMessage(p, msg))
	require.Contains(t, s.consensus.(*fakeConsensus).payloads, msg.Payload.(*payload.Extensible))

	t.Run("small ValidUntilBlockEnd", func(t *testing.T) {
		t.Run("current height", func(t *testing.T) {
			msg := newConsensusMessage(0, s.chain.BlockHeight())
			require.NoError(t, s.handleMessage(p, msg))
			require.NotContains(t, s.consensus.(*fakeConsensus).payloads, msg.Payload.(*payload.Extensible))
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
	t.Run("invalid category", func(t *testing.T) {
		pl := payload.NewExtensible()
		pl.Category = "invalid"
		pl.ValidBlockEnd = s.chain.BlockHeight() + 1
		msg := NewMessage(CMDExtensible, pl)
		require.Error(t, s.handleMessage(p, msg))
	})
}

func TestTransaction(t *testing.T) {
	s := startTestServer(t)

	t.Run("good", func(t *testing.T) {
		tx := newDummyTx()
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
		require.Contains(t, s.consensus.(*fakeConsensus).txs, tx)
	})
	t.Run("bad", func(t *testing.T) {
		tx := newDummyTx()
		s.chain.(*fakechain.FakeChain).PoolTxF = func(*transaction.Transaction) error { return core.ErrInsufficientFunds }
		s.testHandleMessage(t, nil, CMDTX, tx)
		for _, ftx := range s.consensus.(*fakeConsensus).txs {
			require.NotEqual(t, ftx, tx)
		}
	})
}

func (s *Server) testHandleGetData(t *testing.T, invType payload.InventoryType, hs, notFound []util.Uint256, found payload.Payload) {
	var recvResponse atomic.Bool
	var recvNotFound atomic.Bool

	p := newLocalPeer(t, s)
	p.handshaked = true
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

	require.Eventually(t, func() bool { return recvResponse.Load() }, time.Second, time.Millisecond)
	require.Eventually(t, func() bool { return recvNotFound.Load() }, time.Second, time.Millisecond)
}

func TestGetData(t *testing.T) {
	s := startTestServer(t)
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
			Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64)...), VerificationScript: make([]byte, 0)}, {InvocationScript: []byte{}, VerificationScript: []byte{}}},
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
	p.handshaked = true
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
	p.handshaked = true
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
	p.handshaked = true
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
}

func TestInv(t *testing.T) {
	s := startTestServer(t)
	s.chain.(*fakechain.FakeChain).UtilityTokenBalance = big.NewInt(10000000)

	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = true
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
		s.chain.(*fakechain.FakeChain).PutTx(tx)
		hs := []util.Uint256{random.Uint256(), tx.Hash(), random.Uint256()}
		s.testHandleMessage(t, p, CMDInv, &payload.Inventory{
			Type:   payload.TXType,
			Hashes: hs,
		})
		require.Equal(t, []util.Uint256{hs[0], hs[2]}, actual)
	})
	t.Run("extensible", func(t *testing.T) {
		ep := payload.NewExtensible()
		s.chain.(*fakechain.FakeChain).VerifyWitnessF = func() error { return nil }
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

func TestRequestTx(t *testing.T) {
	s := startTestServer(t)

	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = true
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDGetData {
			actual = append(actual, msg.Payload.(*payload.Inventory).Hashes...)
		}
	}
	s.register <- p
	s.register <- p // ensure previous send was handled

	t.Run("no hashes, no message", func(t *testing.T) {
		actual = nil
		s.requestTx()
		require.Nil(t, actual)
	})
	t.Run("good, small", func(t *testing.T) {
		actual = nil
		expected := []util.Uint256{random.Uint256(), random.Uint256()}
		s.requestTx(expected...)
		require.Equal(t, expected, actual)
	})
	t.Run("good, exactly one chunk", func(t *testing.T) {
		actual = nil
		expected := make([]util.Uint256, payload.MaxHashesCount)
		for i := range expected {
			expected[i] = random.Uint256()
		}
		s.requestTx(expected...)
		require.Equal(t, expected, actual)
	})
	t.Run("good, multiple chunks", func(t *testing.T) {
		actual = nil
		expected := make([]util.Uint256, payload.MaxHashesCount*2+payload.MaxHashesCount/2)
		for i := range expected {
			expected[i] = random.Uint256()
		}
		s.requestTx(expected...)
		require.Equal(t, expected, actual)
	})
}

func TestRequestMPTData(t *testing.T) {
	s := startTestServer(t)

	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = true
	p.messageHandler = func(t *testing.T, msg *Message) {
		if msg.Command == CMDGetMPTData {
			actual = append(actual, msg.Payload.(*payload.MPTInventory).Hashes...)
		}
	}
	s.register <- p
	s.register <- p // ensure previous send was handled

	check := func(t *testing.T, expected []util.Uint256) {
		actual = nil
		request := make(map[util.Uint256]bool)
		for _, h := range expected {
			request[h] = true
		}
		require.NoError(t, s.requestMPTNodes(p, request))
		require.ElementsMatch(t, expected, actual)

	}
	t.Run("no hashes, no message", func(t *testing.T) {
		actual = nil
		require.NoError(t, s.requestMPTNodes(p, nil))
		require.Nil(t, actual)
	})
	t.Run("good, small", func(t *testing.T) {
		expected := []util.Uint256{random.Uint256(), random.Uint256()}
		check(t, expected)
	})
	t.Run("good, exactly one chunk", func(t *testing.T) {
		expected := make([]util.Uint256, payload.MaxMPTHashesCount)
		for i := range expected {
			expected[i] = random.Uint256()
		}
		check(t, expected)
	})
	t.Run("good, multiple chunks", func(t *testing.T) {
		expected := make([]util.Uint256, payload.MaxMPTHashesCount*2+payload.MaxMPTHashesCount/2)
		for i := range expected {
			expected[i] = random.Uint256()
		}
		check(t, expected)
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
	p.handshaked = true
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
func (f feerStub) P2PSigExtensionsEnabled() bool                { return false }
func (f feerStub) GetBaseExecFee() int64                        { return interop.DefaultBaseExecFee }

func TestMemPool(t *testing.T) {
	s := startTestServer(t)

	var actual []util.Uint256
	p := newLocalPeer(t, s)
	p.handshaked = true
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
	newNotaryRequest := func() *payload.P2PNotaryRequest {
		return &payload.P2PNotaryRequest{
			MainTransaction: &transaction.Transaction{Script: []byte{0, 1, 2}},
			FallbackTransaction: &transaction.Transaction{
				ValidUntilBlock: 321,
				Signers:         []transaction.Signer{{Account: bc.NotaryContractScriptHash}, {Account: random.Uint160()}},
			},
			Witness: transaction.Witness{},
		}
	}

	t.Run("bad payload witness", func(t *testing.T) {
		bc.VerifyWitnessF = func() error { return errors.New("bad witness") }
		require.Error(t, verifyNotaryRequest(bc, nil, newNotaryRequest()))
	})

	t.Run("bad fallback sender", func(t *testing.T) {
		bc.VerifyWitnessF = func() error { return nil }
		r := newNotaryRequest()
		r.FallbackTransaction.Signers[0] = transaction.Signer{Account: util.Uint160{7, 8, 9}}
		require.Error(t, verifyNotaryRequest(bc, nil, r))
	})

	t.Run("expired deposit", func(t *testing.T) {
		r := newNotaryRequest()
		bc.NotaryDepositExpiration = r.FallbackTransaction.ValidUntilBlock
		require.Error(t, verifyNotaryRequest(bc, nil, r))
	})

	t.Run("good", func(t *testing.T) {
		r := newNotaryRequest()
		bc.NotaryDepositExpiration = r.FallbackTransaction.ValidUntilBlock + 1
		require.NoError(t, verifyNotaryRequest(bc, nil, r))
	})
}
