package network

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	mrand "math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/consensus"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/extpool"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/services/notary"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"github.com/nspcc-dev/neo-go/pkg/services/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

const (
	// peer numbers are arbitrary at the moment.
	defaultMinPeers           = 5
	defaultAttemptConnPeers   = 20
	defaultMaxPeers           = 100
	defaultExtensiblePoolSize = 20
	maxBlockBatch             = 200
	minPoolCount              = 30
)

var (
	errAlreadyConnected = errors.New("already connected")
	errIdenticalID      = errors.New("identical node id")
	errInvalidNetwork   = errors.New("invalid network")
	errMaxPeers         = errors.New("max peers reached")
	errServerShutdown   = errors.New("server shutdown")
	errInvalidInvType   = errors.New("invalid inventory type")
)

type (
	// Server represents the local Node in the network. Its transport could
	// be of any kind.
	Server struct {
		// ServerConfig holds the Server configuration.
		ServerConfig

		// id also known as the nonce of the server.
		id uint32

		// Network's magic number for correct message decoding.
		network netmode.Magic
		// stateRootInHeader specifies if block header contain state root.
		stateRootInHeader bool

		transport         Transporter
		discovery         Discoverer
		chain             blockchainer.Blockchainer
		bQueue            *blockQueue
		consensus         consensus.Service
		notaryRequestPool *mempool.Pool
		extensiblePool    *extpool.Pool
		notaryFeer        NotaryFeer
		notaryModule      *notary.Notary

		lock  sync.RWMutex
		peers map[Peer]bool

		// lastRequestedHeight contains last requested height.
		lastRequestedHeight atomic.Uint32

		register   chan Peer
		unregister chan peerDrop
		quit       chan struct{}

		transactions chan *transaction.Transaction

		syncReached *atomic.Bool

		oracle    *oracle.Oracle
		stateRoot stateroot.Service

		log *zap.Logger
	}

	peerDrop struct {
		peer   Peer
		reason error
	}
)

func randomID() uint32 {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return binary.BigEndian.Uint32(buf)
}

// NewServer returns a new Server, initialized with the given configuration.
func NewServer(config ServerConfig, chain blockchainer.Blockchainer, log *zap.Logger) (*Server, error) {
	return newServerFromConstructors(config, chain, log, func(s *Server) Transporter {
		return NewTCPTransport(s, net.JoinHostPort(s.ServerConfig.Address, strconv.Itoa(int(s.ServerConfig.Port))), s.log)
	}, consensus.NewService, newDefaultDiscovery)
}

func newServerFromConstructors(config ServerConfig, chain blockchainer.Blockchainer, log *zap.Logger,
	newTransport func(*Server) Transporter,
	newConsensus func(consensus.Config) (consensus.Service, error),
	newDiscovery func([]string, time.Duration, Transporter) Discoverer,
) (*Server, error) {
	if log == nil {
		return nil, errors.New("logger is a required parameter")
	}

	if config.ExtensiblePoolSize <= 0 {
		config.ExtensiblePoolSize = defaultExtensiblePoolSize
		log.Info("ExtensiblePoolSize is not set or wrong, using default value",
			zap.Int("ExtensiblePoolSize", config.ExtensiblePoolSize))
	}

	s := &Server{
		ServerConfig:      config,
		chain:             chain,
		id:                randomID(),
		network:           chain.GetConfig().Magic,
		stateRootInHeader: chain.GetConfig().StateRootInHeader,
		quit:              make(chan struct{}),
		register:          make(chan Peer),
		unregister:        make(chan peerDrop),
		peers:             make(map[Peer]bool),
		syncReached:       atomic.NewBool(false),
		extensiblePool:    extpool.New(chain, config.ExtensiblePoolSize),
		log:               log,
		transactions:      make(chan *transaction.Transaction, 64),
	}
	if chain.P2PSigExtensionsEnabled() {
		s.notaryFeer = NewNotaryFeer(chain)
		s.notaryRequestPool = mempool.New(chain.GetConfig().P2PNotaryRequestPayloadPoolSize, 1, true)
		chain.RegisterPostBlock(func(bc blockchainer.Blockchainer, txpool *mempool.Pool, _ *block.Block) {
			s.notaryRequestPool.RemoveStale(func(t *transaction.Transaction) bool {
				return bc.IsTxStillRelevant(t, txpool, true)
			}, s.notaryFeer)
		})
		if config.P2PNotaryCfg.Enabled {
			cfg := notary.Config{
				MainCfg: config.P2PNotaryCfg,
				Chain:   chain,
				Log:     log,
			}
			n, err := notary.NewNotary(cfg, s.network, s.notaryRequestPool, func(tx *transaction.Transaction) error {
				if err := s.RelayTxn(tx); err != nil {
					return fmt.Errorf("can't relay completed notary transaction: hash %s, error: %w", tx.Hash().StringLE(), err)
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create Notary module: %w", err)
			}
			s.notaryModule = n
			chain.SetNotary(n)
		}
	} else if config.P2PNotaryCfg.Enabled {
		return nil, errors.New("P2PSigExtensions are disabled, but Notary service is enabled")
	}
	s.bQueue = newBlockQueue(maxBlockBatch, chain, log, func(b *block.Block) {
		s.tryStartServices()
	})

	if config.StateRootCfg.Enabled && chain.GetConfig().StateRootInHeader {
		return nil, errors.New("`StateRootInHeader` should be disabled when state service is enabled")
	}

	sr, err := stateroot.New(config.StateRootCfg, s.log, chain, s.handleNewPayload)
	if err != nil {
		return nil, fmt.Errorf("can't initialize StateRoot service: %w", err)
	}
	s.stateRoot = sr

	if config.OracleCfg.Enabled {
		orcCfg := oracle.Config{
			Log:     log,
			Network: config.Net,
			MainCfg: config.OracleCfg,
			Chain:   chain,
		}
		orc, err := oracle.NewOracle(orcCfg)
		if err != nil {
			return nil, fmt.Errorf("can't initialize Oracle module: %w", err)
		}
		orc.SetOnTransaction(func(tx *transaction.Transaction) {
			if err := s.RelayTxn(tx); err != nil {
				orc.Log.Error("can't pool oracle tx",
					zap.String("hash", tx.Hash().StringLE()),
					zap.Error(err))
			}
		})
		s.oracle = orc
		chain.SetOracle(orc)
	}

	srv, err := newConsensus(consensus.Config{
		Logger:                log,
		Broadcast:             s.handleNewPayload,
		Chain:                 chain,
		ProtocolConfiguration: chain.GetConfig(),
		RequestTx:             s.requestTx,
		Wallet:                config.Wallet,

		TimePerBlock: config.TimePerBlock,
	})
	if err != nil {
		return nil, err
	}

	s.consensus = srv

	if s.MinPeers < 0 {
		s.log.Info("bad MinPeers configured, using the default value",
			zap.Int("configured", s.MinPeers),
			zap.Int("actual", defaultMinPeers))
		s.MinPeers = defaultMinPeers
	}

	if s.MaxPeers <= 0 {
		s.log.Info("bad MaxPeers configured, using the default value",
			zap.Int("configured", s.MaxPeers),
			zap.Int("actual", defaultMaxPeers))
		s.MaxPeers = defaultMaxPeers
	}

	if s.AttemptConnPeers <= 0 {
		s.log.Info("bad AttemptConnPeers configured, using the default value",
			zap.Int("configured", s.AttemptConnPeers),
			zap.Int("actual", defaultAttemptConnPeers))
		s.AttemptConnPeers = defaultAttemptConnPeers
	}

	s.transport = newTransport(s)
	s.discovery = newDiscovery(
		s.Seeds,
		s.DialTimeout,
		s.transport,
	)

	return s, nil
}

// ID returns the servers ID.
func (s *Server) ID() uint32 {
	return s.id
}

// Start will start the server and its underlying transport.
func (s *Server) Start(errChan chan error) {
	s.log.Info("node started",
		zap.Uint32("blockHeight", s.chain.BlockHeight()),
		zap.Uint32("headerHeight", s.chain.HeaderHeight()))

	s.tryStartServices()
	s.initStaleMemPools()

	go s.broadcastTxLoop()
	go s.relayBlocksLoop()
	go s.bQueue.run()
	go s.transport.Accept()
	setServerAndNodeVersions(s.UserAgent, strconv.FormatUint(uint64(s.id), 10))
	s.run()
}

// Shutdown disconnects all peers and stops listening.
func (s *Server) Shutdown() {
	s.log.Info("shutting down server", zap.Int("peers", s.PeerCount()))
	s.transport.Close()
	s.discovery.Close()
	s.consensus.Shutdown()
	for p := range s.Peers() {
		p.Disconnect(errServerShutdown)
	}
	s.bQueue.discard()
	if s.StateRootCfg.Enabled {
		s.stateRoot.Shutdown()
	}
	if s.oracle != nil {
		s.oracle.Shutdown()
	}
	if s.notaryModule != nil {
		s.notaryModule.Stop()
	}
	if s.chain.P2PSigExtensionsEnabled() {
		s.notaryRequestPool.StopSubscriptions()
	}
	close(s.quit)
}

// GetOracle returns oracle module instance.
func (s *Server) GetOracle() *oracle.Oracle {
	return s.oracle
}

// GetStateRoot returns state root service instance.
func (s *Server) GetStateRoot() stateroot.Service {
	return s.stateRoot
}

// UnconnectedPeers returns a list of peers that are in the discovery peer list
// but are not connected to the server.
func (s *Server) UnconnectedPeers() []string {
	return s.discovery.UnconnectedPeers()
}

// BadPeers returns a list of peers the are flagged as "bad" peers.
func (s *Server) BadPeers() []string {
	return s.discovery.BadPeers()
}

// ConnectedPeers returns a list of currently connected peers.
func (s *Server) ConnectedPeers() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	peers := make([]string, 0, len(s.peers))
	for k := range s.peers {
		peers = append(peers, k.PeerAddr().String())
	}

	return peers
}

// run is a goroutine that starts another goroutine to manage protocol specifics
// while itself dealing with peers management (handling connects/disconnects).
func (s *Server) run() {
	go s.runProto()
	for {
		if s.PeerCount() < s.MinPeers {
			s.discovery.RequestRemote(s.AttemptConnPeers)
		}
		if s.discovery.PoolCount() < minPoolCount {
			s.broadcastHPMessage(NewMessage(CMDGetAddr, payload.NewNullPayload()))
		}
		select {
		case <-s.quit:
			return
		case p := <-s.register:
			s.lock.Lock()
			s.peers[p] = true
			s.lock.Unlock()
			peerCount := s.PeerCount()
			s.log.Info("new peer connected", zap.Stringer("addr", p.RemoteAddr()), zap.Int("peerCount", peerCount))
			if peerCount > s.MaxPeers {
				s.lock.RLock()
				// Pick a random peer and drop connection to it.
				for peer := range s.peers {
					// It will send us unregister signal.
					go peer.Disconnect(errMaxPeers)
					break
				}
				s.lock.RUnlock()
			}
			updatePeersConnectedMetric(s.PeerCount())

		case drop := <-s.unregister:
			s.lock.Lock()
			if s.peers[drop.peer] {
				delete(s.peers, drop.peer)
				s.lock.Unlock()
				s.log.Warn("peer disconnected",
					zap.Stringer("addr", drop.peer.RemoteAddr()),
					zap.String("reason", drop.reason.Error()),
					zap.Int("peerCount", s.PeerCount()))
				addr := drop.peer.PeerAddr().String()
				if drop.reason == errIdenticalID {
					s.discovery.RegisterBadAddr(addr)
				} else if drop.reason == errAlreadyConnected {
					// There is a race condition when peer can be disconnected twice for the this reason
					// which can lead to no connections to peer at all. Here we check for such a possibility.
					stillConnected := false
					s.lock.RLock()
					verDrop := drop.peer.Version()
					addr := drop.peer.PeerAddr().String()
					if verDrop != nil {
						for peer := range s.peers {
							ver := peer.Version()
							// Already connected, drop this connection.
							if ver != nil && ver.Nonce == verDrop.Nonce && peer.PeerAddr().String() == addr {
								stillConnected = true
							}
						}
					}
					s.lock.RUnlock()
					if !stillConnected {
						s.discovery.UnregisterConnectedAddr(addr)
						s.discovery.BackFill(addr)
					}
				} else {
					s.discovery.UnregisterConnectedAddr(addr)
					s.discovery.BackFill(addr)
				}
				updatePeersConnectedMetric(s.PeerCount())
			} else {
				// else the peer is already gone, which can happen
				// because we have two goroutines sending signals here
				s.lock.Unlock()
			}
		}
	}
}

// runProto is a goroutine that manages server-wide protocol events.
func (s *Server) runProto() {
	pingTimer := time.NewTimer(s.PingInterval)
	for {
		prevHeight := s.chain.BlockHeight()
		select {
		case <-s.quit:
			return
		case <-pingTimer.C:
			if s.chain.BlockHeight() == prevHeight {
				// Get a copy of s.peers to avoid holding a lock while sending.
				for peer := range s.Peers() {
					_ = peer.SendPing(NewMessage(CMDPing, payload.NewPing(s.id, s.chain.HeaderHeight())))
				}
			}
			pingTimer.Reset(s.PingInterval)
		}
	}
}

func (s *Server) tryStartServices() {
	if s.syncReached.Load() {
		return
	}

	if s.IsInSync() && s.syncReached.CAS(false, true) {
		s.log.Info("node reached synchronized state, starting services")
		if s.Wallet != nil {
			s.consensus.Start()
		}
		if s.StateRootCfg.Enabled {
			s.stateRoot.Run()
		}
		if s.oracle != nil {
			go s.oracle.Run()
		}
		if s.chain.P2PSigExtensionsEnabled() {
			s.notaryRequestPool.RunSubscriptions() // WSClient is also a subscriber.
		}
		if s.notaryModule != nil {
			go s.notaryModule.Run()
		}
	}
}

// SubscribeForNotaryRequests adds given channel to a notary request event
// broadcasting, so when a new P2PNotaryRequest is received or an existing
// P2PNotaryRequest is removed from pool you'll receive it via this channel.
// Make sure it's read from regularly as not reading these events might affect
// other Server functions.
// Ensure that P2PSigExtensions are enabled before calling this method.
func (s *Server) SubscribeForNotaryRequests(ch chan<- mempoolevent.Event) {
	if !s.chain.P2PSigExtensionsEnabled() {
		panic("P2PSigExtensions are disabled")
	}
	s.notaryRequestPool.SubscribeForTransactions(ch)
}

// UnsubscribeFromNotaryRequests unsubscribes given channel from notary request
// notifications, you can close it afterwards. Passing non-subscribed channel
// is a no-op.
// Ensure that P2PSigExtensions are enabled before calling this method.
func (s *Server) UnsubscribeFromNotaryRequests(ch chan<- mempoolevent.Event) {
	if !s.chain.P2PSigExtensionsEnabled() {
		panic("P2PSigExtensions are disabled")
	}
	s.notaryRequestPool.UnsubscribeFromTransactions(ch)
}

// Peers returns the current list of peers connected to
// the server.
func (s *Server) Peers() map[Peer]bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	peers := make(map[Peer]bool, len(s.peers))
	for k, v := range s.peers {
		peers[k] = v
	}

	return peers
}

// PeerCount returns the number of current connected peers.
func (s *Server) PeerCount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.peers)
}

// HandshakedPeersCount returns the number of connected peers
// which have already performed handshake.
func (s *Server) HandshakedPeersCount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var count int

	for p := range s.peers {
		if p.Handshaked() {
			count++
		}
	}

	return count
}

// getVersionMsg returns current version message.
func (s *Server) getVersionMsg() (*Message, error) {
	port, err := s.Port()
	if err != nil {
		return nil, err
	}

	capabilities := []capability.Capability{
		{
			Type: capability.TCPServer,
			Data: &capability.Server{
				Port: port,
			},
		},
	}
	if s.Relay {
		capabilities = append(capabilities, capability.Capability{
			Type: capability.FullNode,
			Data: &capability.Node{
				StartHeight: s.chain.BlockHeight(),
			},
		})
	}
	payload := payload.NewVersion(
		s.Net,
		s.id,
		s.UserAgent,
		capabilities,
	)
	return NewMessage(CMDVersion, payload), nil
}

// IsInSync answers the question of whether the server is in sync with the
// network or not (at least how the server itself sees it). The server operates
// with the data that it has, the number of peers (that has to be more than
// minimum number) and height of these peers (our chain has to be not lower
// than 2/3 of our peers have). Ideally we would check for the highest of the
// peers, but the problem is that they can lie to us and send whatever height
// they want to.
func (s *Server) IsInSync() bool {
	var peersNumber int
	var notHigher int

	if s.MinPeers == 0 {
		return true
	}

	ourLastBlock := s.chain.BlockHeight()

	s.lock.RLock()
	for p := range s.peers {
		if p.Handshaked() {
			peersNumber++
			if ourLastBlock >= p.LastBlockIndex() {
				notHigher++
			}
		}
	}
	s.lock.RUnlock()

	// Checking bQueue would also be nice, but it can be filled with garbage
	// easily at the moment.
	return peersNumber >= s.MinPeers && (3*notHigher > 2*peersNumber) // && s.bQueue.length() == 0
}

// When a peer sends out his version we reply with verack after validating
// the version.
func (s *Server) handleVersionCmd(p Peer, version *payload.Version) error {
	err := p.HandleVersion(version)
	if err != nil {
		return err
	}
	if s.id == version.Nonce {
		return errIdenticalID
	}
	// Make sure both server and peer are operating on
	// the same network.
	if s.Net != version.Magic {
		return errInvalidNetwork
	}
	peerAddr := p.PeerAddr().String()
	s.discovery.RegisterConnectedAddr(peerAddr)
	s.lock.RLock()
	for peer := range s.peers {
		if p == peer {
			continue
		}
		ver := peer.Version()
		// Already connected, drop this connection.
		if ver != nil && ver.Nonce == version.Nonce && peer.PeerAddr().String() == peerAddr {
			s.lock.RUnlock()
			return errAlreadyConnected
		}
	}
	s.lock.RUnlock()
	return p.SendVersionAck(NewMessage(CMDVerack, payload.NewNullPayload()))
}

// handleBlockCmd processes the received block received from its peer.
func (s *Server) handleBlockCmd(p Peer, block *block.Block) error {
	return s.bQueue.putBlock(block)
}

// handlePing processes ping request.
func (s *Server) handlePing(p Peer, ping *payload.Ping) error {
	err := p.HandlePing(ping)
	if err != nil {
		return err
	}
	if s.chain.BlockHeight() < ping.LastBlockIndex {
		err = s.requestBlocks(p)
		if err != nil {
			return err
		}
	}
	return p.EnqueueP2PMessage(NewMessage(CMDPong, payload.NewPing(s.chain.BlockHeight(), s.id)))
}

// handlePing processes pong request.
func (s *Server) handlePong(p Peer, pong *payload.Ping) error {
	err := p.HandlePong(pong)
	if err != nil {
		return err
	}
	if s.chain.BlockHeight() < pong.LastBlockIndex {
		return s.requestBlocks(p)
	}
	return nil
}

// handleInvCmd processes the received inventory.
func (s *Server) handleInvCmd(p Peer, inv *payload.Inventory) error {
	reqHashes := make([]util.Uint256, 0)
	var typExists = map[payload.InventoryType]func(util.Uint256) bool{
		payload.TXType:    s.chain.HasTransaction,
		payload.BlockType: s.chain.HasBlock,
		payload.ExtensibleType: func(h util.Uint256) bool {
			cp := s.extensiblePool.Get(h)
			return cp != nil
		},
		payload.P2PNotaryRequestType: func(h util.Uint256) bool {
			return s.notaryRequestPool.ContainsKey(h)
		},
	}
	if exists := typExists[inv.Type]; exists != nil {
		for _, hash := range inv.Hashes {
			if !exists(hash) {
				reqHashes = append(reqHashes, hash)
			}
		}
	}
	if len(reqHashes) > 0 {
		msg := NewMessage(CMDGetData, payload.NewInventory(inv.Type, reqHashes))
		pkt, err := msg.Bytes()
		if err != nil {
			return err
		}
		if inv.Type == payload.ExtensibleType {
			return p.EnqueueHPPacket(true, pkt)
		}
		return p.EnqueueP2PPacket(pkt)
	}
	return nil
}

// handleMempoolCmd handles getmempool command.
func (s *Server) handleMempoolCmd(p Peer) error {
	txs := s.chain.GetMemPool().GetVerifiedTransactions()
	hs := make([]util.Uint256, 0, payload.MaxHashesCount)
	for i := range txs {
		hs = append(hs, txs[i].Hash())
		if len(hs) < payload.MaxHashesCount && i != len(txs)-1 {
			continue
		}
		msg := NewMessage(CMDInv, payload.NewInventory(payload.TXType, hs))
		err := p.EnqueueP2PMessage(msg)
		if err != nil {
			return err
		}
		hs = hs[:0]
	}
	return nil
}

// handleInvCmd processes the received inventory.
func (s *Server) handleGetDataCmd(p Peer, inv *payload.Inventory) error {
	var notFound []util.Uint256
	for _, hash := range inv.Hashes {
		var msg *Message

		switch inv.Type {
		case payload.TXType:
			tx, _, err := s.chain.GetTransaction(hash)
			if err == nil {
				msg = NewMessage(CMDTX, tx)
			} else {
				notFound = append(notFound, hash)
			}
		case payload.BlockType:
			b, err := s.chain.GetBlock(hash)
			if err == nil {
				msg = NewMessage(CMDBlock, b)
			} else {
				notFound = append(notFound, hash)
			}
		case payload.ExtensibleType:
			if cp := s.extensiblePool.Get(hash); cp != nil {
				msg = NewMessage(CMDExtensible, cp)
			}
		case payload.P2PNotaryRequestType:
			if nrp, ok := s.notaryRequestPool.TryGetData(hash); ok { // already have checked P2PSigExtEnabled
				msg = NewMessage(CMDP2PNotaryRequest, nrp.(*payload.P2PNotaryRequest))
			} else {
				notFound = append(notFound, hash)
			}
		}
		if msg != nil {
			pkt, err := msg.Bytes()
			if err == nil {
				if inv.Type == payload.ExtensibleType {
					err = p.EnqueueHPPacket(true, pkt)
				} else {
					err = p.EnqueueP2PPacket(pkt)
				}
			}
			if err != nil {
				return err
			}
		}
	}
	if len(notFound) != 0 {
		return p.EnqueueP2PMessage(NewMessage(CMDNotFound, payload.NewInventory(inv.Type, notFound)))
	}
	return nil
}

// handleGetBlocksCmd processes the getblocks request.
func (s *Server) handleGetBlocksCmd(p Peer, gb *payload.GetBlocks) error {
	count := gb.Count
	if gb.Count < 0 || gb.Count > payload.MaxHashesCount {
		count = payload.MaxHashesCount
	}
	start, err := s.chain.GetHeader(gb.HashStart)
	if err != nil {
		return err
	}
	blockHashes := make([]util.Uint256, 0)
	for i := start.Index + 1; i <= start.Index+uint32(count); i++ {
		hash := s.chain.GetHeaderHash(int(i))
		if hash.Equals(util.Uint256{}) {
			break
		}
		blockHashes = append(blockHashes, hash)
	}

	if len(blockHashes) == 0 {
		return nil
	}
	payload := payload.NewInventory(payload.BlockType, blockHashes)
	msg := NewMessage(CMDInv, payload)
	return p.EnqueueP2PMessage(msg)
}

// handleGetBlockByIndexCmd processes the getblockbyindex request.
func (s *Server) handleGetBlockByIndexCmd(p Peer, gbd *payload.GetBlockByIndex) error {
	count := gbd.Count
	if gbd.Count < 0 || gbd.Count > payload.MaxHashesCount {
		count = payload.MaxHashesCount
	}
	for i := gbd.IndexStart; i < gbd.IndexStart+uint32(count); i++ {
		hash := s.chain.GetHeaderHash(int(i))
		if hash.Equals(util.Uint256{}) {
			break
		}
		b, err := s.chain.GetBlock(hash)
		if err != nil {
			break
		}
		msg := NewMessage(CMDBlock, b)
		if err = p.EnqueueP2PMessage(msg); err != nil {
			return err
		}
	}
	return nil
}

// handleGetHeadersCmd processes the getheaders request.
func (s *Server) handleGetHeadersCmd(p Peer, gh *payload.GetBlockByIndex) error {
	if gh.IndexStart > s.chain.HeaderHeight() {
		return nil
	}
	count := gh.Count
	if gh.Count < 0 || gh.Count > payload.MaxHeadersAllowed {
		count = payload.MaxHeadersAllowed
	}
	resp := payload.Headers{}
	resp.Hdrs = make([]*block.Header, 0, count)
	for i := gh.IndexStart; i < gh.IndexStart+uint32(count); i++ {
		hash := s.chain.GetHeaderHash(int(i))
		if hash.Equals(util.Uint256{}) {
			break
		}
		header, err := s.chain.GetHeader(hash)
		if err != nil {
			break
		}
		resp.Hdrs = append(resp.Hdrs, header)
	}
	if len(resp.Hdrs) == 0 {
		return nil
	}
	msg := NewMessage(CMDHeaders, &resp)
	return p.EnqueueP2PMessage(msg)
}

// handleExtensibleCmd processes received extensible payload.
func (s *Server) handleExtensibleCmd(e *payload.Extensible) error {
	if !s.syncReached.Load() {
		return nil
	}
	ok, err := s.extensiblePool.Add(e)
	if err != nil {
		return err
	}
	if !ok { // payload is already in cache
		return nil
	}
	switch e.Category {
	case consensus.Category:
		s.consensus.OnPayload(e)
	case stateroot.Category:
		err := s.stateRoot.OnPayload(e)
		if err != nil {
			return err
		}
	default:
		return errors.New("invalid category")
	}

	msg := NewMessage(CMDInv, payload.NewInventory(payload.ExtensibleType, []util.Uint256{e.Hash()}))
	if e.Category == consensus.Category {
		s.broadcastHPMessage(msg)
	} else {
		s.broadcastMessage(msg)
	}
	return nil
}

// handleTxCmd processes received transaction.
// It never returns an error.
func (s *Server) handleTxCmd(tx *transaction.Transaction) error {
	// It's OK for it to fail for various reasons like tx already existing
	// in the pool.
	if s.verifyAndPoolTX(tx) == nil {
		s.consensus.OnTransaction(tx)
		s.broadcastTX(tx, nil)
	}
	return nil
}

// handleP2PNotaryRequestCmd process received P2PNotaryRequest payload.
func (s *Server) handleP2PNotaryRequestCmd(r *payload.P2PNotaryRequest) error {
	if !s.chain.P2PSigExtensionsEnabled() {
		return errors.New("P2PNotaryRequestCMD was received, but P2PSignatureExtensions are disabled")
	}
	// It's OK for it to fail for various reasons like request already existing
	// in the pool.
	_ = s.RelayP2PNotaryRequest(r)
	return nil
}

// RelayP2PNotaryRequest adds given request to the pool and relays. It does not check
// P2PSigExtensions enabled.
func (s *Server) RelayP2PNotaryRequest(r *payload.P2PNotaryRequest) error {
	err := s.verifyAndPoolNotaryRequest(r)
	if err == nil {
		s.broadcastP2PNotaryRequestPayload(nil, r)
	}
	return err
}

// verifyAndPoolNotaryRequest verifies NotaryRequest payload and adds it to the payload mempool.
func (s *Server) verifyAndPoolNotaryRequest(r *payload.P2PNotaryRequest) error {
	return s.chain.PoolTxWithData(r.FallbackTransaction, r, s.notaryRequestPool, s.notaryFeer, verifyNotaryRequest)
}

// verifyNotaryRequest is a function for state-dependant P2PNotaryRequest payload verification which is executed before ordinary blockchain's verification.
func verifyNotaryRequest(bc blockchainer.Blockchainer, _ *transaction.Transaction, data interface{}) error {
	r := data.(*payload.P2PNotaryRequest)
	payer := r.FallbackTransaction.Signers[1].Account
	if err := bc.VerifyWitness(payer, r, &r.Witness, bc.GetPolicer().GetMaxVerificationGAS()); err != nil {
		return fmt.Errorf("bad P2PNotaryRequest payload witness: %w", err)
	}
	notaryHash := bc.GetNotaryContractScriptHash()
	if r.FallbackTransaction.Sender() != notaryHash {
		return errors.New("P2PNotary contract should be a sender of the fallback transaction")
	}
	depositExpiration := bc.GetNotaryDepositExpiration(payer)
	if r.FallbackTransaction.ValidUntilBlock >= depositExpiration {
		return fmt.Errorf("fallback transaction is valid after deposit is unlocked: ValidUntilBlock is %d, deposit lock expires at %d", r.FallbackTransaction.ValidUntilBlock, depositExpiration)
	}
	return nil
}

func (s *Server) broadcastP2PNotaryRequestPayload(_ *transaction.Transaction, data interface{}) {
	r := data.(*payload.P2PNotaryRequest) // we can guarantee that cast is successful
	msg := NewMessage(CMDInv, payload.NewInventory(payload.P2PNotaryRequestType, []util.Uint256{r.FallbackTransaction.Hash()}))
	s.broadcastMessage(msg)
}

// handleAddrCmd will process received addresses.
func (s *Server) handleAddrCmd(p Peer, addrs *payload.AddressList) error {
	if !p.CanProcessAddr() {
		return errors.New("unexpected addr received")
	}
	dups := make(map[string]bool)
	for _, a := range addrs.Addrs {
		addr, err := a.GetTCPAddress()
		if err == nil && !dups[addr] {
			dups[addr] = true
			s.discovery.BackFill(addr)
		}
	}
	return nil
}

// handleGetAddrCmd sends to the peer some good addresses that we know of.
func (s *Server) handleGetAddrCmd(p Peer) error {
	addrs := s.discovery.GoodPeers()
	if len(addrs) > payload.MaxAddrsCount {
		addrs = addrs[:payload.MaxAddrsCount]
	}
	alist := payload.NewAddressList(len(addrs))
	ts := time.Now()
	for i, addr := range addrs {
		// we know it's a good address, so it can't fail
		netaddr, _ := net.ResolveTCPAddr("tcp", addr.Address)
		alist.Addrs[i] = payload.NewAddressAndTime(netaddr, ts, addr.Capabilities)
	}
	return p.EnqueueP2PMessage(NewMessage(CMDAddr, alist))
}

// requestBlocks sends a CMDGetBlockByIndex message to the peer
// to sync up in blocks. A maximum of maxBlockBatch will
// send at once. Two things we need to take care of:
// 1. If possible, blocks should be fetched in parallel.
//    height..+500 to one peer, height+500..+1000 to another etc.
// 2. Every block must eventually be fetched even if peer sends no answer.
// Thus the following algorithm is used:
// 1. Block range is divided into chunks of payload.MaxHashesCount.
// 2. Send requests for chunk in increasing order.
// 3. After all requests were sent, request random height.
func (s *Server) requestBlocks(p Peer) error {
	var currHeight = s.chain.BlockHeight()
	var peerHeight = p.LastBlockIndex()
	var needHeight uint32
	// lastRequestedHeight can only be increased.
	for {
		old := s.lastRequestedHeight.Load()
		if old <= currHeight {
			needHeight = currHeight + 1
			if !s.lastRequestedHeight.CAS(old, needHeight) {
				continue
			}
		} else if old < currHeight+(blockCacheSize-payload.MaxHashesCount) {
			needHeight = currHeight + 1
			if peerHeight > old+payload.MaxHashesCount {
				needHeight = old + payload.MaxHashesCount
				if !s.lastRequestedHeight.CAS(old, needHeight) {
					continue
				}
			}
		} else {
			index := mrand.Intn(blockCacheSize / payload.MaxHashesCount)
			needHeight = currHeight + 1 + uint32(index*payload.MaxHashesCount)
		}
		break
	}
	payload := payload.NewGetBlockByIndex(needHeight, -1)
	return p.EnqueueP2PMessage(NewMessage(CMDGetBlockByIndex, payload))
}

// handleMessage processes the given message.
func (s *Server) handleMessage(peer Peer, msg *Message) error {
	s.log.Debug("got msg",
		zap.Stringer("addr", peer.RemoteAddr()),
		zap.String("type", msg.Command.String()))

	if peer.Handshaked() {
		if inv, ok := msg.Payload.(*payload.Inventory); ok {
			if !inv.Type.Valid(s.chain.P2PSigExtensionsEnabled()) || len(inv.Hashes) == 0 {
				return errInvalidInvType
			}
		}
		switch msg.Command {
		case CMDAddr:
			addrs := msg.Payload.(*payload.AddressList)
			return s.handleAddrCmd(peer, addrs)
		case CMDGetAddr:
			// it has no payload
			return s.handleGetAddrCmd(peer)
		case CMDGetBlocks:
			gb := msg.Payload.(*payload.GetBlocks)
			return s.handleGetBlocksCmd(peer, gb)
		case CMDGetBlockByIndex:
			gbd := msg.Payload.(*payload.GetBlockByIndex)
			return s.handleGetBlockByIndexCmd(peer, gbd)
		case CMDGetData:
			inv := msg.Payload.(*payload.Inventory)
			return s.handleGetDataCmd(peer, inv)
		case CMDGetHeaders:
			gh := msg.Payload.(*payload.GetBlockByIndex)
			return s.handleGetHeadersCmd(peer, gh)
		case CMDInv:
			inventory := msg.Payload.(*payload.Inventory)
			return s.handleInvCmd(peer, inventory)
		case CMDMempool:
			// no payload
			return s.handleMempoolCmd(peer)
		case CMDBlock:
			block := msg.Payload.(*block.Block)
			return s.handleBlockCmd(peer, block)
		case CMDExtensible:
			cp := msg.Payload.(*payload.Extensible)
			return s.handleExtensibleCmd(cp)
		case CMDTX:
			tx := msg.Payload.(*transaction.Transaction)
			return s.handleTxCmd(tx)
		case CMDP2PNotaryRequest:
			r := msg.Payload.(*payload.P2PNotaryRequest)
			return s.handleP2PNotaryRequestCmd(r)
		case CMDPing:
			ping := msg.Payload.(*payload.Ping)
			return s.handlePing(peer, ping)
		case CMDPong:
			pong := msg.Payload.(*payload.Ping)
			return s.handlePong(peer, pong)
		case CMDVersion, CMDVerack:
			return fmt.Errorf("received '%s' after the handshake", msg.Command.String())
		}
	} else {
		switch msg.Command {
		case CMDVersion:
			version := msg.Payload.(*payload.Version)
			return s.handleVersionCmd(peer, version)
		case CMDVerack:
			err := peer.HandleVersionAck()
			if err != nil {
				return err
			}
			go peer.StartProtocol()

			s.tryStartServices()
		default:
			return fmt.Errorf("received '%s' during handshake", msg.Command.String())
		}
	}
	return nil
}

func (s *Server) handleNewPayload(p *payload.Extensible) {
	_, err := s.extensiblePool.Add(p)
	if err != nil {
		s.log.Error("created payload is not valid", zap.Error(err))
		return
	}

	msg := NewMessage(CMDInv, payload.NewInventory(payload.ExtensibleType, []util.Uint256{p.Hash()}))
	switch p.Category {
	case consensus.Category:
		// It's high priority because it directly affects consensus process,
		// even though it's just an inv.
		s.broadcastHPMessage(msg)
	default:
		s.broadcastMessage(msg)
	}
}

func (s *Server) requestTx(hashes ...util.Uint256) {
	if len(hashes) == 0 {
		return
	}

	for i := 0; i <= len(hashes)/payload.MaxHashesCount; i++ {
		start := i * payload.MaxHashesCount
		stop := (i + 1) * payload.MaxHashesCount
		if stop > len(hashes) {
			stop = len(hashes)
		}
		if start == stop {
			break
		}
		msg := NewMessage(CMDGetData, payload.NewInventory(payload.TXType, hashes[start:stop]))
		// It's high priority because it directly affects consensus process,
		// even though it's getdata.
		s.broadcastHPMessage(msg)
	}
}

// iteratePeersWithSendMsg sends given message to all peers using two functions
// passed, one is to send the message and the other is to filtrate peers (the
// peer is considered invalid if it returns false).
func (s *Server) iteratePeersWithSendMsg(msg *Message, send func(Peer, bool, []byte) error, peerOK func(Peer) bool) {
	// Get a copy of s.peers to avoid holding a lock while sending.
	peers := s.Peers()
	if len(peers) == 0 {
		return
	}
	pkt, err := msg.Bytes()
	if err != nil {
		return
	}

	success := make(map[Peer]bool, len(peers))
	okCount := 0
	sentCount := 0
	for peer := range peers {
		if peerOK != nil && !peerOK(peer) {
			success[peer] = false
			continue
		}
		okCount++
		if err := send(peer, false, pkt); err != nil {
			continue
		}
		if msg.Command == CMDGetAddr {
			peer.AddGetAddrSent()
		}
		success[peer] = true
		sentCount++
	}

	// Send to at least 2/3 of good peers.
	if 3*sentCount >= 2*okCount {
		return
	}

	// Perform blocking send now.
	for peer := range peers {
		if _, ok := success[peer]; ok || peerOK != nil && !peerOK(peer) {
			continue
		}
		if err := send(peer, true, pkt); err != nil {
			continue
		}
		if msg.Command == CMDGetAddr {
			peer.AddGetAddrSent()
		}
		sentCount++
		if 3*sentCount >= 2*okCount {
			return
		}
	}
}

// broadcastMessage sends the message to all available peers.
func (s *Server) broadcastMessage(msg *Message) {
	s.iteratePeersWithSendMsg(msg, Peer.EnqueuePacket, nil)
}

// broadcastHPMessage sends the high-priority message to all available peers.
func (s *Server) broadcastHPMessage(msg *Message) {
	s.iteratePeersWithSendMsg(msg, Peer.EnqueueHPPacket, nil)
}

// relayBlocksLoop subscribes to new blocks in the ledger and broadcasts them
// to the network. Intended to be run as a separate goroutine.
func (s *Server) relayBlocksLoop() {
	ch := make(chan *block.Block, 2) // Some buffering to smooth out possible egressing delays.
	s.chain.SubscribeForBlocks(ch)
	for {
		select {
		case <-s.quit:
			s.chain.UnsubscribeFromBlocks(ch)
			return
		case b := <-ch:
			msg := NewMessage(CMDInv, payload.NewInventory(payload.BlockType, []util.Uint256{b.Hash()}))
			// Filter out nodes that are more current (avoid spamming the network
			// during initial sync).
			s.iteratePeersWithSendMsg(msg, Peer.EnqueuePacket, func(p Peer) bool {
				return p.Handshaked() && p.LastBlockIndex() < b.Index
			})
			s.extensiblePool.RemoveStale(b.Index)
		}
	}
}

// verifyAndPoolTX verifies the TX and adds it to the local mempool.
func (s *Server) verifyAndPoolTX(t *transaction.Transaction) error {
	return s.chain.PoolTx(t)
}

// RelayTxn a new transaction to the local node and the connected peers.
// Reference: the method OnRelay in C#: https://github.com/neo-project/neo/blob/master/neo/Network/P2P/LocalNode.cs#L159
func (s *Server) RelayTxn(t *transaction.Transaction) error {
	err := s.verifyAndPoolTX(t)
	if err == nil {
		s.broadcastTX(t, nil)
	}
	return err
}

// broadcastTX broadcasts an inventory message about new transaction.
func (s *Server) broadcastTX(t *transaction.Transaction, _ interface{}) {
	select {
	case s.transactions <- t:
	case <-s.quit:
	}
}

func (s *Server) broadcastTxHashes(hs []util.Uint256) {
	msg := NewMessage(CMDInv, payload.NewInventory(payload.TXType, hs))

	// We need to filter out non-relaying nodes, so plain broadcast
	// functions don't fit here.
	s.iteratePeersWithSendMsg(msg, Peer.EnqueuePacket, Peer.IsFullNode)
}

// initStaleMemPools initializes mempools for stale tx/payload processing.
func (s *Server) initStaleMemPools() {
	cfg := s.chain.GetConfig()
	threshold := 5
	if cfg.ValidatorsCount*2 > threshold {
		threshold = cfg.ValidatorsCount * 2
	}

	mp := s.chain.GetMemPool()
	mp.SetResendThreshold(uint32(threshold), s.broadcastTX)
	if s.chain.P2PSigExtensionsEnabled() {
		s.notaryRequestPool.SetResendThreshold(uint32(threshold), s.broadcastP2PNotaryRequestPayload)
	}
}

// broadcastTxLoop is a loop for batching and sending
// transactions hashes in an INV payload.
func (s *Server) broadcastTxLoop() {
	const (
		batchTime = time.Millisecond * 50
		batchSize = 32
	)

	txs := make([]util.Uint256, 0, batchSize)
	var timer *time.Timer

	timerCh := func() <-chan time.Time {
		if timer == nil {
			return nil
		}
		return timer.C
	}

	broadcast := func() {
		s.broadcastTxHashes(txs)
		txs = txs[:0]
		if timer != nil {
			timer.Stop()
		}
	}

	for {
		select {
		case <-s.quit:
		loop:
			for {
				select {
				case <-s.transactions:
				default:
					break loop
				}
			}
			return
		case <-timerCh():
			if len(txs) > 0 {
				broadcast()
			}
		case tx := <-s.transactions:
			if len(txs) == 0 {
				timer = time.NewTimer(batchTime)
			}

			txs = append(txs, tx.Hash())
			if len(txs) == batchSize {
				broadcast()
			}
		}
	}
}

// Port returns a server port that should be used in P2P version exchange. In
// case if `AnnouncedPort` is set in the server.Config, the announced node port
// will be returned (e.g. consider the node running behind NAT). If `AnnouncedPort`
// isn't set, the port returned may still differs from that of server.Config.
func (s *Server) Port() (uint16, error) {
	if s.AnnouncedPort != 0 {
		return s.ServerConfig.AnnouncedPort, nil
	}
	var port uint16
	_, portStr, err := net.SplitHostPort(s.transport.Address())
	if err != nil {
		port = s.ServerConfig.Port
	} else {
		p, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return 0, err
		}
		port = uint16(p)
	}
	return port, nil
}
