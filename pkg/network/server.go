package network

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	mrand "math/rand"
	"net"
	"runtime"
	"sort"
	"strconv"
	"sync"
	satomic "sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/extpool"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
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
	defaultBroadcastFactor    = 0
	maxBlockBatch             = 200
	peerTimeFactor            = 1000
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
	// Ledger is everything Server needs from the blockchain.
	Ledger interface {
		extpool.Ledger
		mempool.Feer
		Blockqueuer
		GetBlock(hash util.Uint256) (*block.Block, error)
		GetConfig() config.Blockchain
		GetHeader(hash util.Uint256) (*block.Header, error)
		GetHeaderHash(uint32) util.Uint256
		GetMaxVerificationGAS() int64
		GetMemPool() *mempool.Pool
		GetNotaryBalance(acc util.Uint160) *big.Int
		GetNotaryContractScriptHash() util.Uint160
		GetNotaryDepositExpiration(acc util.Uint160) uint32
		GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
		HasBlock(util.Uint256) bool
		HeaderHeight() uint32
		P2PSigExtensionsEnabled() bool
		PoolTx(t *transaction.Transaction, pools ...*mempool.Pool) error
		PoolTxWithData(t *transaction.Transaction, data interface{}, mp *mempool.Pool, feer mempool.Feer, verificationFunction func(t *transaction.Transaction, data interface{}) error) error
		RegisterPostBlock(f func(func(*transaction.Transaction, *mempool.Pool, bool) bool, *mempool.Pool, *block.Block))
		SubscribeForBlocks(ch chan *block.Block)
		UnsubscribeFromBlocks(ch chan *block.Block)
	}

	// Service is a service abstraction (oracle, state root, consensus, etc).
	Service interface {
		Name() string
		Start()
		Shutdown()
	}

	// Server represents the local Node in the network. Its transport could
	// be of any kind.
	Server struct {
		// ServerConfig holds the Server configuration.
		ServerConfig

		// id also known as the nonce of the server.
		id uint32

		// A copy of the Ledger's config.
		config config.ProtocolConfiguration

		transports        []Transporter
		discovery         Discoverer
		chain             Ledger
		bQueue            *blockQueue
		bSyncQueue        *blockQueue
		mempool           *mempool.Pool
		notaryRequestPool *mempool.Pool
		extensiblePool    *extpool.Pool
		notaryFeer        NotaryFeer

		serviceLock    sync.RWMutex
		services       map[string]Service
		extensHandlers map[string]func(*payload.Extensible) error
		txCallback     func(*transaction.Transaction)
		txCbList       satomic.Value

		txInLock sync.RWMutex
		txin     chan *transaction.Transaction
		txInMap  map[util.Uint256]struct{}

		lock  sync.RWMutex
		peers map[Peer]bool

		// lastRequestedBlock contains a height of the last requested block.
		lastRequestedBlock atomic.Uint32
		// lastRequestedHeader contains a height of the last requested header.
		lastRequestedHeader atomic.Uint32

		register   chan Peer
		unregister chan peerDrop
		handshake  chan Peer
		quit       chan struct{}
		relayFin   chan struct{}

		transactions chan *transaction.Transaction

		syncReached *atomic.Bool

		stateSync StateSync

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
func NewServer(config ServerConfig, chain Ledger, stSync StateSync, log *zap.Logger) (*Server, error) {
	return newServerFromConstructors(config, chain, stSync, log, func(s *Server, addr string) Transporter {
		return NewTCPTransport(s, addr, s.log)
	}, newDefaultDiscovery)
}

func newServerFromConstructors(config ServerConfig, chain Ledger, stSync StateSync, log *zap.Logger,
	newTransport func(*Server, string) Transporter,
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
		ServerConfig:   config,
		chain:          chain,
		id:             randomID(),
		config:         chain.GetConfig().ProtocolConfiguration,
		quit:           make(chan struct{}),
		relayFin:       make(chan struct{}),
		register:       make(chan Peer),
		unregister:     make(chan peerDrop),
		handshake:      make(chan Peer),
		txInMap:        make(map[util.Uint256]struct{}),
		peers:          make(map[Peer]bool),
		syncReached:    atomic.NewBool(false),
		mempool:        chain.GetMemPool(),
		extensiblePool: extpool.New(chain, config.ExtensiblePoolSize),
		log:            log,
		txin:           make(chan *transaction.Transaction, 64),
		transactions:   make(chan *transaction.Transaction, 64),
		services:       make(map[string]Service),
		extensHandlers: make(map[string]func(*payload.Extensible) error),
		stateSync:      stSync,
	}
	if chain.P2PSigExtensionsEnabled() {
		s.notaryFeer = NewNotaryFeer(chain)
		s.notaryRequestPool = mempool.New(s.config.P2PNotaryRequestPayloadPoolSize, 1, true)
		chain.RegisterPostBlock(func(isRelevant func(*transaction.Transaction, *mempool.Pool, bool) bool, txpool *mempool.Pool, _ *block.Block) {
			s.notaryRequestPool.RemoveStale(func(t *transaction.Transaction) bool {
				return isRelevant(t, txpool, true)
			}, s.notaryFeer)
		})
	}
	s.bQueue = newBlockQueue(chain, log, func(b *block.Block) {
		s.tryStartServices()
	})

	s.bSyncQueue = newBlockQueue(s.stateSync, log, nil)

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

	if s.BroadcastFactor < 0 || s.BroadcastFactor > 100 {
		s.log.Info("bad BroadcastFactor configured, using the default value",
			zap.Int("configured", s.BroadcastFactor),
			zap.Int("actual", defaultBroadcastFactor))
		s.BroadcastFactor = defaultBroadcastFactor
	}

	if len(s.ServerConfig.Addresses) == 0 {
		return nil, errors.New("no bind addresses configured")
	}
	transports := make([]Transporter, len(s.ServerConfig.Addresses))
	for i, addr := range s.ServerConfig.Addresses {
		transports[i] = newTransport(s, addr.Address)
	}
	s.transports = transports
	s.discovery = newDiscovery(
		s.Seeds,
		s.DialTimeout,
		// Here we need to pick up a single transporter, it will be used to
		// dial, and it doesn't matter which one.
		s.transports[0],
	)

	return s, nil
}

// ID returns the servers ID.
func (s *Server) ID() uint32 {
	return s.id
}

// Start will start the server and its underlying transport. Calling it twice
// is an error.
func (s *Server) Start(errChan chan error) {
	s.log.Info("node started",
		zap.Uint32("blockHeight", s.chain.BlockHeight()),
		zap.Uint32("headerHeight", s.chain.HeaderHeight()))

	s.tryStartServices()
	s.initStaleMemPools()

	var txThreads = optimalNumOfThreads()
	for i := 0; i < txThreads; i++ {
		go s.txHandlerLoop()
	}
	go s.broadcastTxLoop()
	go s.relayBlocksLoop()
	go s.bQueue.run()
	go s.bSyncQueue.run()
	for _, tr := range s.transports {
		go tr.Accept()
	}
	setServerAndNodeVersions(s.UserAgent, strconv.FormatUint(uint64(s.id), 10))
	s.run()
}

// Shutdown disconnects all peers and stops listening. Calling it twice is an error,
// once stopped the same intance of the Server can't be started again by calling Start.
func (s *Server) Shutdown() {
	s.log.Info("shutting down server", zap.Int("peers", s.PeerCount()))
	for _, tr := range s.transports {
		tr.Close()
	}
	for _, p := range s.getPeers(nil) {
		p.Disconnect(errServerShutdown)
	}
	s.bQueue.discard()
	s.bSyncQueue.discard()
	s.serviceLock.RLock()
	for _, svc := range s.services {
		svc.Shutdown()
	}
	s.serviceLock.RUnlock()
	if s.chain.P2PSigExtensionsEnabled() {
		s.notaryRequestPool.StopSubscriptions()
	}
	close(s.quit)
	<-s.relayFin
}

// AddService allows to add a service to be started/stopped by Server.
func (s *Server) AddService(svc Service) {
	s.serviceLock.Lock()
	defer s.serviceLock.Unlock()
	s.addService(svc)
}

// addService is an unlocked version of AddService.
func (s *Server) addService(svc Service) {
	s.services[svc.Name()] = svc
}

// AddExtensibleService register a service that handles an extensible payload of some kind.
func (s *Server) AddExtensibleService(svc Service, category string, handler func(*payload.Extensible) error) {
	s.serviceLock.Lock()
	defer s.serviceLock.Unlock()
	s.addExtensibleService(svc, category, handler)
}

// addExtensibleService is an unlocked version of AddExtensibleService.
func (s *Server) addExtensibleService(svc Service, category string, handler func(*payload.Extensible) error) {
	s.extensHandlers[category] = handler
	s.addService(svc)
}

// AddConsensusService registers consensus service that handles transactions and dBFT extensible payloads.
func (s *Server) AddConsensusService(svc Service, handler func(*payload.Extensible) error, txCallback func(*transaction.Transaction)) {
	s.serviceLock.Lock()
	defer s.serviceLock.Unlock()
	s.txCallback = txCallback
	s.addExtensibleService(svc, payload.ConsensusCategory, handler)
}

// DelService drops a service from the list, use it when the service is stopped
// outside of the Server.
func (s *Server) DelService(svc Service) {
	s.serviceLock.Lock()
	defer s.serviceLock.Unlock()
	s.delService(svc)
}

// delService is an unlocked version of DelService.
func (s *Server) delService(svc Service) {
	delete(s.services, svc.Name())
}

// DelExtensibleService drops a service that handler extensible payloads from the
// list, use it when the service is stopped outside of the Server.
func (s *Server) DelExtensibleService(svc Service, category string) {
	s.serviceLock.Lock()
	defer s.serviceLock.Unlock()
	s.delExtensibleService(svc, category)
}

// delExtensibleService is an unlocked version of DelExtensibleService.
func (s *Server) delExtensibleService(svc Service, category string) {
	delete(s.extensHandlers, category)
	s.delService(svc)
}

// DelConsensusService unregisters consensus service that handles transactions and dBFT extensible payloads.
func (s *Server) DelConsensusService(svc Service) {
	s.serviceLock.Lock()
	defer s.serviceLock.Unlock()
	s.txCallback = nil
	s.delExtensibleService(svc, payload.ConsensusCategory)
}

// GetNotaryPool allows to retrieve notary pool, if it's configured.
func (s *Server) GetNotaryPool() *mempool.Pool {
	return s.notaryRequestPool
}

// UnconnectedPeers returns a list of peers that are in the discovery peer list
// but are not connected to the server.
func (s *Server) UnconnectedPeers() []string {
	return s.discovery.UnconnectedPeers()
}

// BadPeers returns a list of peers that are flagged as "bad" peers.
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
	var (
		peerCheckTime    = s.TimePerBlock * peerTimeFactor
		addrCheckTimeout bool
		addrTimer        = time.NewTimer(peerCheckTime)
		peerTimer        = time.NewTimer(s.ProtoTickInterval)
	)
	defer addrTimer.Stop()
	defer peerTimer.Stop()
	go s.runProto()
	for loopCnt := 0; ; loopCnt++ {
		var (
			netSize = s.discovery.NetworkSize()
			// "Optimal" number of peers.
			optimalN = s.discovery.GetFanOut() * 2
			// Real number of peers.
			peerN = s.HandshakedPeersCount()
			// Timeout value for the next peerTimer, long one by default.
			peerT = peerCheckTime
		)

		if peerN < s.MinPeers {
			// Starting up or going below the minimum -> quickly get many new peers.
			s.discovery.RequestRemote(s.AttemptConnPeers)
			// Check/retry new connections soon.
			peerT = s.ProtoTickInterval
		} else if s.MinPeers > 0 && loopCnt%s.MinPeers == 0 && optimalN > peerN && optimalN < s.MaxPeers && optimalN < netSize {
			// Having some number of peers, but probably can get some more, the network is big.
			// It also allows to start picking up new peers proactively, before we suddenly have <s.MinPeers of them.
			var connN = s.AttemptConnPeers
			if connN > optimalN-peerN {
				connN = optimalN - peerN
			}
			s.discovery.RequestRemote(connN)
		}

		if addrCheckTimeout || s.discovery.PoolCount() < s.AttemptConnPeers {
			s.broadcastHPMessage(NewMessage(CMDGetAddr, payload.NewNullPayload()))
			addrCheckTimeout = false
		}
		select {
		case <-s.quit:
			return
		case <-addrTimer.C:
			addrCheckTimeout = true
			addrTimer.Reset(peerCheckTime)
		case <-peerTimer.C:
			peerTimer.Reset(peerT)
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
					zap.Error(drop.reason),
					zap.Int("peerCount", s.PeerCount()))
				if errors.Is(drop.reason, errIdenticalID) {
					s.discovery.RegisterSelf(drop.peer)
				} else {
					s.discovery.UnregisterConnected(drop.peer, errors.Is(drop.reason, errAlreadyConnected))
				}
				updatePeersConnectedMetric(s.PeerCount())
			} else {
				// else the peer is already gone, which can happen
				// because we have two goroutines sending signals here
				s.lock.Unlock()
			}

		case p := <-s.handshake:
			ver := p.Version()
			s.log.Info("started protocol",
				zap.Stringer("addr", p.RemoteAddr()),
				zap.ByteString("userAgent", ver.UserAgent),
				zap.Uint32("startHeight", p.LastBlockIndex()),
				zap.Uint32("id", ver.Nonce))

			s.discovery.RegisterGood(p)

			s.tryInitStateSync()
			s.tryStartServices()
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
				s.broadcastMessage(NewMessage(CMDPing, payload.NewPing(s.chain.BlockHeight(), s.id)))
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
		if s.chain.P2PSigExtensionsEnabled() {
			s.notaryRequestPool.RunSubscriptions() // WSClient is also a subscriber.
		}
		s.serviceLock.RLock()
		for _, svc := range s.services {
			svc.Start()
		}
		s.serviceLock.RUnlock()
	}
}

// SubscribeForNotaryRequests adds the given channel to a notary request event
// broadcasting, so when a new P2PNotaryRequest is received or an existing
// P2PNotaryRequest is removed from the pool you'll receive it via this channel.
// Make sure it's read from regularly as not reading these events might affect
// other Server functions. Make sure you're not changing the received mempool
// events, as it may affect the functionality of Blockchain and other subscribers.
// Ensure that P2PSigExtensions are enabled before calling this method.
func (s *Server) SubscribeForNotaryRequests(ch chan<- mempoolevent.Event) {
	if !s.chain.P2PSigExtensionsEnabled() {
		panic("P2PSigExtensions are disabled")
	}
	s.notaryRequestPool.SubscribeForTransactions(ch)
}

// UnsubscribeFromNotaryRequests unsubscribes the given channel from notary request
// notifications, you can close it afterwards. Passing non-subscribed channel
// is a no-op.
// Ensure that P2PSigExtensions are enabled before calling this method.
func (s *Server) UnsubscribeFromNotaryRequests(ch chan<- mempoolevent.Event) {
	if !s.chain.P2PSigExtensionsEnabled() {
		panic("P2PSigExtensions are disabled")
	}
	s.notaryRequestPool.UnsubscribeFromTransactions(ch)
}

// getPeers returns the current list of the peers connected to the server filtered by
// isOK function if it's given.
func (s *Server) getPeers(isOK func(Peer) bool) []Peer {
	s.lock.RLock()
	defer s.lock.RUnlock()

	peers := make([]Peer, 0, len(s.peers))
	for k := range s.peers {
		if isOK != nil && !isOK(k) {
			continue
		}
		peers = append(peers, k)
	}

	return peers
}

// PeerCount returns the number of the currently connected peers.
func (s *Server) PeerCount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.peers)
}

// HandshakedPeersCount returns the number of the connected peers
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

// getVersionMsg returns the current version message generated for the specified
// connection.
func (s *Server) getVersionMsg(localAddr net.Addr) (*Message, error) {
	port, err := s.Port(localAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch server port: %w", err)
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
// minimum number) and the height of these peers (our chain has to be not lower
// than 2/3 of our peers have). Ideally, we would check for the highest of the
// peers, but the problem is that they can lie to us and send whatever height
// they want to. Once sync reached, IsInSync will always return `true`, even if
// server is temporary out of sync after that.
func (s *Server) IsInSync() bool {
	if s.syncReached.Load() {
		return true
	}
	var peersNumber int
	var notHigher int

	if s.stateSync.IsActive() {
		return false
	}

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

// When a peer sends out its version, we reply with verack after validating
// the version.
func (s *Server) handleVersionCmd(p Peer, version *payload.Version) error {
	err := p.HandleVersion(version)
	if err != nil {
		return err
	}
	if s.id == version.Nonce {
		return errIdenticalID
	}
	// Make sure both the server and the peer are operating on
	// the same network.
	if s.Net != version.Magic {
		return errInvalidNetwork
	}
	peerAddr := p.PeerAddr().String()
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

// handleBlockCmd processes the block received from its peer.
func (s *Server) handleBlockCmd(p Peer, block *block.Block) error {
	if s.stateSync.IsActive() {
		return s.bSyncQueue.putBlock(block)
	}
	return s.bQueue.putBlock(block)
}

// handlePing processes a ping request.
func (s *Server) handlePing(p Peer, ping *payload.Ping) error {
	err := p.HandlePing(ping)
	if err != nil {
		return err
	}
	err = s.requestBlocksOrHeaders(p)
	if err != nil {
		return err
	}
	return p.EnqueueP2PMessage(NewMessage(CMDPong, payload.NewPing(s.chain.BlockHeight(), s.id)))
}

func (s *Server) requestBlocksOrHeaders(p Peer) error {
	if s.stateSync.NeedHeaders() {
		if s.chain.HeaderHeight() < p.LastBlockIndex() {
			return s.requestHeaders(p)
		}
		return nil
	}
	var (
		bq              Blockqueuer = s.chain
		requestMPTNodes bool
	)
	if s.stateSync.IsActive() {
		bq = s.stateSync
		requestMPTNodes = s.stateSync.NeedMPTNodes()
	}
	if bq.BlockHeight() >= p.LastBlockIndex() {
		return nil
	}
	err := s.requestBlocks(bq, p)
	if err != nil {
		return err
	}
	if requestMPTNodes {
		return s.requestMPTNodes(p, s.stateSync.GetUnknownMPTNodesBatch(payload.MaxMPTHashesCount))
	}
	return nil
}

// requestHeaders sends a CMDGetHeaders message to the peer to sync up in headers.
func (s *Server) requestHeaders(p Peer) error {
	pl := getRequestBlocksPayload(p, s.chain.HeaderHeight(), &s.lastRequestedHeader)
	return p.EnqueueP2PMessage(NewMessage(CMDGetHeaders, pl))
}

// handlePing processes a pong request.
func (s *Server) handlePong(p Peer, pong *payload.Ping) error {
	err := p.HandlePong(pong)
	if err != nil {
		return err
	}
	return s.requestBlocksOrHeaders(p)
}

// handleInvCmd processes the received inventory.
func (s *Server) handleInvCmd(p Peer, inv *payload.Inventory) error {
	var reqHashes = inv.Hashes[:0]
	var typExists = map[payload.InventoryType]func(util.Uint256) bool{
		payload.TXType: func(h util.Uint256) bool {
			s.txInLock.RLock()
			_, ok := s.txInMap[h]
			s.txInLock.RUnlock()
			return ok || s.mempool.ContainsKey(h)
		},
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
		if inv.Type == payload.ExtensibleType {
			return p.EnqueueHPMessage(msg)
		}
		return p.EnqueueP2PMessage(msg)
	}
	return nil
}

// handleMempoolCmd handles getmempool command.
func (s *Server) handleMempoolCmd(p Peer) error {
	txs := s.mempool.GetVerifiedTransactions()
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
	var (
		err      error
		notFound []util.Uint256
		reply    = io.NewBufBinWriter()
		send     = p.EnqueueP2PPacket
	)
	if inv.Type == payload.ExtensibleType {
		send = p.EnqueueHPPacket
	}
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
			err = addMessageToPacket(reply, msg, send)
			if err != nil {
				return err
			}
		}
	}
	if len(notFound) != 0 {
		err = addMessageToPacket(reply, NewMessage(CMDNotFound, payload.NewInventory(inv.Type, notFound)), send)
		if err != nil {
			return err
		}
	}
	if reply.Len() == 0 {
		return nil
	}
	return send(reply.Bytes())
}

// addMessageToPacket serializes given message into the given buffer and sends whole
// batch if it exceeds MaxSize/2 memory limit (to prevent DoS).
func addMessageToPacket(batch *io.BufBinWriter, msg *Message, send func([]byte) error) error {
	err := msg.Encode(batch.BinWriter)
	if err != nil {
		return err
	}
	if batch.Len() > payload.MaxSize/2 {
		err = send(batch.Bytes())
		if err != nil {
			return err
		}
		batch.Reset()
	}
	return nil
}

// handleGetMPTDataCmd processes the received MPT inventory.
func (s *Server) handleGetMPTDataCmd(p Peer, inv *payload.MPTInventory) error {
	if !s.config.P2PStateExchangeExtensions {
		return errors.New("GetMPTDataCMD was received, but P2PStateExchangeExtensions are disabled")
	}
	// Even if s.config.KeepOnlyLatestState enabled, we'll keep latest P1 and P2 MPT states.
	resp := payload.MPTData{}
	capLeft := payload.MaxSize - 8 // max(io.GetVarSize(len(resp.Nodes)))
	added := make(map[util.Uint256]struct{})
	for _, h := range inv.Hashes {
		if capLeft <= 2 { // at least 1 byte for len(nodeBytes) and 1 byte for node type
			break
		}
		err := s.stateSync.Traverse(h,
			func(n mpt.Node, node []byte) bool {
				if _, ok := added[n.Hash()]; ok {
					return false
				}
				l := len(node)
				size := l + io.GetVarSize(l)
				if size > capLeft {
					return true
				}
				resp.Nodes = append(resp.Nodes, node)
				added[n.Hash()] = struct{}{}
				capLeft -= size
				return false
			})
		if err != nil {
			return fmt.Errorf("failed to traverse MPT starting from %s: %w", h.StringBE(), err)
		}
	}
	if len(resp.Nodes) > 0 {
		msg := NewMessage(CMDMPTData, &resp)
		return p.EnqueueP2PMessage(msg)
	}
	return nil
}

func (s *Server) handleMPTDataCmd(p Peer, data *payload.MPTData) error {
	if !s.config.P2PStateExchangeExtensions {
		return errors.New("MPTDataCMD was received, but P2PStateExchangeExtensions are disabled")
	}
	return s.stateSync.AddMPTNodes(data.Nodes)
}

// requestMPTNodes requests the specified MPT nodes from the peer or broadcasts
// request if no peer is specified.
func (s *Server) requestMPTNodes(p Peer, itms []util.Uint256) error {
	if len(itms) == 0 {
		return nil
	}
	if len(itms) > payload.MaxMPTHashesCount {
		itms = itms[:payload.MaxMPTHashesCount]
	}
	pl := payload.NewMPTInventory(itms)
	msg := NewMessage(CMDGetMPTData, pl)
	return p.EnqueueP2PMessage(msg)
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
		hash := s.chain.GetHeaderHash(i)
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
	var reply = io.NewBufBinWriter()
	count := gbd.Count
	if gbd.Count < 0 || gbd.Count > payload.MaxHashesCount {
		count = payload.MaxHashesCount
	}
	for i := gbd.IndexStart; i < gbd.IndexStart+uint32(count); i++ {
		hash := s.chain.GetHeaderHash(i)
		if hash.Equals(util.Uint256{}) {
			break
		}
		b, err := s.chain.GetBlock(hash)
		if err != nil {
			break
		}
		err = addMessageToPacket(reply, NewMessage(CMDBlock, b), p.EnqueueP2PPacket)
		if err != nil {
			return err
		}
	}
	if reply.Len() == 0 {
		return nil
	}
	return p.EnqueueP2PPacket(reply.Bytes())
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
		hash := s.chain.GetHeaderHash(i)
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

// handleHeadersCmd processes headers payload.
func (s *Server) handleHeadersCmd(p Peer, h *payload.Headers) error {
	return s.stateSync.AddHeaders(h.Hdrs...)
}

// handleExtensibleCmd processes the received extensible payload.
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
	s.serviceLock.RLock()
	handler := s.extensHandlers[e.Category]
	s.serviceLock.RUnlock()
	if handler != nil {
		err = handler(e)
		if err != nil {
			return err
		}
	}
	s.advertiseExtensible(e)
	return nil
}

func (s *Server) advertiseExtensible(e *payload.Extensible) {
	msg := NewMessage(CMDInv, payload.NewInventory(payload.ExtensibleType, []util.Uint256{e.Hash()}))
	if e.Category == payload.ConsensusCategory {
		// It's high priority because it directly affects consensus process,
		// even though it's just an inv.
		s.broadcastHPMessage(msg)
	} else {
		s.broadcastMessage(msg)
	}
}

// handleTxCmd processes the received transaction.
// It never returns an error.
func (s *Server) handleTxCmd(tx *transaction.Transaction) error {
	// It's OK for it to fail for various reasons like tx already existing
	// in the pool.
	s.txInLock.Lock()
	_, ok := s.txInMap[tx.Hash()]
	if ok || s.mempool.ContainsKey(tx.Hash()) {
		s.txInLock.Unlock()
		return nil
	}
	s.txInMap[tx.Hash()] = struct{}{}
	s.txInLock.Unlock()
	s.txin <- tx
	return nil
}

func (s *Server) txHandlerLoop() {
txloop:
	for {
		select {
		case tx := <-s.txin:
			s.serviceLock.RLock()
			txCallback := s.txCallback
			s.serviceLock.RUnlock()
			if txCallback != nil {
				var cbList = s.txCbList.Load()
				if cbList != nil {
					var list = cbList.([]util.Uint256)
					var i = sort.Search(len(list), func(i int) bool {
						return list[i].CompareTo(tx.Hash()) >= 0
					})
					if i < len(list) && list[i].Equals(tx.Hash()) {
						txCallback(tx)
					}
				}
			}
			if s.verifyAndPoolTX(tx) == nil {
				s.broadcastTX(tx, nil)
			}
			s.txInLock.Lock()
			delete(s.txInMap, tx.Hash())
			s.txInLock.Unlock()
		case <-s.quit:
			break txloop
		}
	}
drainloop:
	for {
		select {
		case <-s.txin:
		default:
			break drainloop
		}
	}
}

// handleP2PNotaryRequestCmd process the received P2PNotaryRequest payload.
func (s *Server) handleP2PNotaryRequestCmd(r *payload.P2PNotaryRequest) error {
	if !s.chain.P2PSigExtensionsEnabled() {
		return errors.New("P2PNotaryRequestCMD was received, but P2PSignatureExtensions are disabled")
	}
	// It's OK for it to fail for various reasons like request already existing
	// in the pool.
	_ = s.RelayP2PNotaryRequest(r)
	return nil
}

// RelayP2PNotaryRequest adds the given request to the pool and relays. It does not check
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
	return s.chain.PoolTxWithData(r.FallbackTransaction, r, s.notaryRequestPool, s.notaryFeer, s.verifyNotaryRequest)
}

// verifyNotaryRequest is a function for state-dependant P2PNotaryRequest payload verification which is executed before ordinary blockchain's verification.
func (s *Server) verifyNotaryRequest(_ *transaction.Transaction, data interface{}) error {
	r := data.(*payload.P2PNotaryRequest)
	payer := r.FallbackTransaction.Signers[1].Account
	if _, err := s.chain.VerifyWitness(payer, r, &r.Witness, s.chain.GetMaxVerificationGAS()); err != nil {
		return fmt.Errorf("bad P2PNotaryRequest payload witness: %w", err)
	}
	notaryHash := s.chain.GetNotaryContractScriptHash()
	if r.FallbackTransaction.Sender() != notaryHash {
		return fmt.Errorf("P2PNotary contract should be a sender of the fallback transaction, got %s", address.Uint160ToString(r.FallbackTransaction.Sender()))
	}
	depositExpiration := s.chain.GetNotaryDepositExpiration(payer)
	if r.FallbackTransaction.ValidUntilBlock >= depositExpiration {
		return fmt.Errorf("fallback transaction is valid after deposit is unlocked: ValidUntilBlock is %d, deposit lock for %s expires at %d", r.FallbackTransaction.ValidUntilBlock, address.Uint160ToString(payer), depositExpiration)
	}
	return nil
}

func (s *Server) broadcastP2PNotaryRequestPayload(_ *transaction.Transaction, data interface{}) {
	r := data.(*payload.P2PNotaryRequest) // we can guarantee that cast is successful
	msg := NewMessage(CMDInv, payload.NewInventory(payload.P2PNotaryRequestType, []util.Uint256{r.FallbackTransaction.Hash()}))
	s.broadcastMessage(msg)
}

// handleAddrCmd will process the received addresses.
func (s *Server) handleAddrCmd(p Peer, addrs *payload.AddressList) error {
	if !p.CanProcessAddr() {
		return errors.New("unexpected addr received")
	}
	for _, a := range addrs.Addrs {
		addr, err := a.GetTCPAddress()
		if err == nil {
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
// to sync up in blocks. A maximum of maxBlockBatch will be
// sent at once. There are two things we need to take care of:
//  1. If possible, blocks should be fetched in parallel.
//     height..+500 to one peer, height+500..+1000 to another etc.
//  2. Every block must eventually be fetched even if the peer sends no answer.
//
// Thus, the following algorithm is used:
// 1. Block range is divided into chunks of payload.MaxHashesCount.
// 2. Send requests for chunk in increasing order.
// 3. After all requests have been sent, request random height.
func (s *Server) requestBlocks(bq Blockqueuer, p Peer) error {
	pl := getRequestBlocksPayload(p, bq.BlockHeight(), &s.lastRequestedBlock)
	lq, capLeft := s.bQueue.lastQueued()
	if capLeft == 0 {
		// No more blocks will fit into the queue.
		return nil
	}
	if lq >= pl.IndexStart {
		if capLeft < payload.MaxHashesCount {
			pl.Count = int16(capLeft)
		}
		pl.IndexStart = lq + 1
	}
	return p.EnqueueP2PMessage(NewMessage(CMDGetBlockByIndex, pl))
}

func getRequestBlocksPayload(p Peer, currHeight uint32, lastRequestedHeight *atomic.Uint32) *payload.GetBlockByIndex {
	var peerHeight = p.LastBlockIndex()
	var needHeight uint32
	// lastRequestedBlock can only be increased.
	for {
		old := lastRequestedHeight.Load()
		if old <= currHeight {
			needHeight = currHeight + 1
			if !lastRequestedHeight.CAS(old, needHeight) {
				continue
			}
		} else if old < currHeight+(blockCacheSize-payload.MaxHashesCount) {
			needHeight = currHeight + 1
			if peerHeight > old+payload.MaxHashesCount {
				needHeight = old + payload.MaxHashesCount
				if !lastRequestedHeight.CAS(old, needHeight) {
					continue
				}
			}
		} else {
			index := mrand.Intn(blockCacheSize / payload.MaxHashesCount)
			needHeight = currHeight + 1 + uint32(index*payload.MaxHashesCount)
		}
		break
	}
	return payload.NewGetBlockByIndex(needHeight, -1)
}

// handleMessage processes the given message.
func (s *Server) handleMessage(peer Peer, msg *Message) error {
	s.log.Debug("got msg",
		zap.Stringer("addr", peer.RemoteAddr()),
		zap.Stringer("type", msg.Command))

	start := time.Now()
	defer func() { addCmdTimeMetric(msg.Command, time.Since(start)) }()

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
		case CMDGetMPTData:
			inv := msg.Payload.(*payload.MPTInventory)
			return s.handleGetMPTDataCmd(peer, inv)
		case CMDMPTData:
			inv := msg.Payload.(*payload.MPTData)
			return s.handleMPTDataCmd(peer, inv)
		case CMDGetHeaders:
			gh := msg.Payload.(*payload.GetBlockByIndex)
			return s.handleGetHeadersCmd(peer, gh)
		case CMDHeaders:
			h := msg.Payload.(*payload.Headers)
			return s.handleHeadersCmd(peer, h)
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
		default:
			return fmt.Errorf("received '%s' during handshake", msg.Command.String())
		}
	}
	return nil
}

func (s *Server) tryInitStateSync() {
	if !s.stateSync.IsActive() {
		s.bSyncQueue.discard()
		return
	}

	if s.stateSync.IsInitialized() {
		return
	}

	var peersNumber int
	s.lock.RLock()
	heights := make([]uint32, 0)
	for p := range s.peers {
		if p.Handshaked() {
			peersNumber++
			peerLastBlock := p.LastBlockIndex()
			i := sort.Search(len(heights), func(i int) bool {
				return heights[i] >= peerLastBlock
			})
			heights = append(heights, peerLastBlock)
			if i != len(heights)-1 {
				copy(heights[i+1:], heights[i:])
				heights[i] = peerLastBlock
			}
		}
	}
	s.lock.RUnlock()
	if peersNumber >= s.MinPeers && len(heights) > 0 {
		// choose the height of the median peer as the current chain's height
		h := heights[len(heights)/2]
		err := s.stateSync.Init(h)
		if err != nil {
			s.log.Fatal("failed to init state sync module",
				zap.Uint32("evaluated chain's blockHeight", h),
				zap.Uint32("blockHeight", s.chain.BlockHeight()),
				zap.Uint32("headerHeight", s.chain.HeaderHeight()),
				zap.Error(err))
		}

		// module can be inactive after init (i.e. full state is collected and ordinary block processing is needed)
		if !s.stateSync.IsActive() {
			s.bSyncQueue.discard()
		}
	}
}

// BroadcastExtensible add a locally-generated Extensible payload to the pool
// and advertises it to peers.
func (s *Server) BroadcastExtensible(p *payload.Extensible) {
	_, err := s.extensiblePool.Add(p)
	if err != nil {
		s.log.Error("created payload is not valid", zap.Error(err))
		return
	}

	s.advertiseExtensible(p)
}

// RequestTx asks for the given transactions from Server peers using GetData message.
func (s *Server) RequestTx(hashes ...util.Uint256) {
	if len(hashes) == 0 {
		return
	}

	var sorted = make([]util.Uint256, len(hashes))
	copy(sorted, hashes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CompareTo(sorted[j]) < 0
	})

	s.txCbList.Store(sorted)

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

// StopTxFlow makes the server not call previously specified consensus transaction callback.
func (s *Server) StopTxFlow() {
	var hashes []util.Uint256
	s.txCbList.Store(hashes)
}

// iteratePeersWithSendMsg sends the given message to all peers using two functions
// passed, one is to send the message and the other is to filtrate peers (the
// peer is considered invalid if it returns false).
func (s *Server) iteratePeersWithSendMsg(msg *Message, send func(Peer, context.Context, []byte) error, peerOK func(Peer) bool) {
	var deadN, peerN, sentN int

	// Get a copy of s.peers to avoid holding a lock while sending.
	peers := s.getPeers(peerOK)
	peerN = len(peers)
	if peerN == 0 {
		return
	}
	pkt, err := msg.Bytes()
	if err != nil {
		return
	}

	var (
		// Optimal number of recipients.
		enoughN     = s.discovery.GetFanOut()
		replies     = make(chan error, peerN) // Cache is there just to make goroutines exit faster.
		ctx, cancel = context.WithTimeout(context.Background(), s.TimePerBlock/2)
	)
	enoughN = (enoughN*(100-s.BroadcastFactor) + peerN*s.BroadcastFactor) / 100
	for _, peer := range peers {
		go func(p Peer, ctx context.Context, pkt []byte) {
			// Do this before packet is sent, reader thread can get the reply before this routine wakes up.
			if msg.Command == CMDGetAddr {
				p.AddGetAddrSent()
			}
			if msg.Command == CMDPing {
				p.SetPingTimer()
			}
			replies <- send(p, ctx, pkt)
		}(peer, ctx, pkt)
	}
	for r := range replies {
		if r == nil {
			sentN++
		} else {
			deadN++
		}
		if sentN+deadN == peerN {
			break
		}
		if sentN >= enoughN && ctx.Err() == nil {
			cancel()
		}
	}
	cancel()
	close(replies)
}

// broadcastMessage sends the message to all available peers.
func (s *Server) broadcastMessage(msg *Message) {
	s.iteratePeersWithSendMsg(msg, Peer.BroadcastPacket, Peer.Handshaked)
}

// broadcastHPMessage sends the high-priority message to all available peers.
func (s *Server) broadcastHPMessage(msg *Message) {
	s.iteratePeersWithSendMsg(msg, Peer.BroadcastHPPacket, Peer.Handshaked)
}

// relayBlocksLoop subscribes to new blocks in the ledger and broadcasts them
// to the network. Intended to be run as a separate goroutine.
func (s *Server) relayBlocksLoop() {
	ch := make(chan *block.Block, 2) // Some buffering to smooth out possible egressing delays.
	s.chain.SubscribeForBlocks(ch)
mainloop:
	for {
		select {
		case <-s.quit:
			s.chain.UnsubscribeFromBlocks(ch)
			break mainloop
		case b := <-ch:
			msg := NewMessage(CMDInv, payload.NewInventory(payload.BlockType, []util.Uint256{b.Hash()}))
			// Filter out nodes that are more current (avoid spamming the network
			// during initial sync).
			s.iteratePeersWithSendMsg(msg, Peer.BroadcastPacket, func(p Peer) bool {
				return p.Handshaked() && p.LastBlockIndex() < b.Index
			})
			s.extensiblePool.RemoveStale(b.Index)
		}
	}
drainBlocksLoop:
	for {
		select {
		case <-ch:
		default:
			break drainBlocksLoop
		}
	}
	close(ch)
	close(s.relayFin)
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
	s.iteratePeersWithSendMsg(msg, Peer.BroadcastPacket, Peer.IsFullNode)
}

// initStaleMemPools initializes mempools for stale tx/payload processing.
func (s *Server) initStaleMemPools() {
	threshold := 5
	// Not perfect, can change over time, but should be sufficient.
	numOfCNs := s.config.GetNumOfCNs(s.chain.BlockHeight())
	if numOfCNs*2 > threshold {
		threshold = numOfCNs * 2
	}

	s.mempool.SetResendThreshold(uint32(threshold), s.broadcastTX)
	if s.chain.P2PSigExtensionsEnabled() {
		s.notaryRequestPool.SetResendThreshold(uint32(threshold), s.broadcastP2PNotaryRequestPayload)
	}
}

// broadcastTxLoop is a loop for batching and sending
// transactions hashes in an INV payload.
func (s *Server) broadcastTxLoop() {
	const (
		batchTime = time.Millisecond * 50
		batchSize = 42
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

// Port returns a server port that should be used in P2P version exchange with the
// peer connected on the given localAddr. In case if announced node port is set
// in the server.Config for the given bind address, the announced node port will
// be returned (e.g. consider the node running behind NAT). If `AnnouncedPort`
// isn't set, the port returned may still differ from that of server.Config. If
// no localAddr is given, then the first available port will be returned.
func (s *Server) Port(localAddr net.Addr) (uint16, error) {
	var connIP string
	if localAddr != nil {
		connIP, _, _ = net.SplitHostPort(localAddr.String()) // Ignore error and provide info if possible.
	}
	var defaultPort *uint16
	for i, tr := range s.transports {
		listenIP, listenPort := tr.HostPort()
		if listenIP == "::" || listenIP == "" || localAddr == nil || connIP == "" || connIP == listenIP {
			var res uint16
			if s.ServerConfig.Addresses[i].AnnouncedPort != 0 {
				res = s.ServerConfig.Addresses[i].AnnouncedPort
			} else {
				p, err := strconv.ParseUint(listenPort, 10, 16)
				if err != nil {
					return 0, fmt.Errorf("failed to parse bind port from '%s': %w", listenPort, err)
				}
				res = uint16(p)
			}
			if localAddr == nil || // no local address is specified => take the first port available
				(listenIP != "::" && listenIP != "") { // direct match is always preferable
				return res, nil
			}
			defaultPort = &res
		}
	}
	if defaultPort != nil {
		return *defaultPort, nil
	}
	return 0, fmt.Errorf("bind address for connection '%s' is not registered", localAddr.String())
}

// optimalNumOfThreads returns the optimal number of processing threads to create
// for transaction processing.
func optimalNumOfThreads() int {
	// Doing more won't help, mempool is still a contention point.
	const maxThreads = 16
	var threads = runtime.GOMAXPROCS(0)
	if threads > runtime.NumCPU() {
		threads = runtime.NumCPU()
	}
	if threads > maxThreads {
		threads = maxThreads
	}
	return threads
}
