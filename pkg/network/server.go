package network

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/consensus"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

const (
	// peer numbers are arbitrary at the moment.
	defaultMinPeers         = 5
	defaultAttemptConnPeers = 20
	defaultMaxPeers         = 100
	maxBlockBatch           = 200
	maxAddrsToSend          = 200
	minPoolCount            = 30
)

var (
	errAlreadyConnected = errors.New("already connected")
	errIdenticalID      = errors.New("identical node id")
	errInvalidHandshake = errors.New("invalid handshake")
	errInvalidNetwork   = errors.New("invalid network")
	errMaxPeers         = errors.New("max peers reached")
	errServerShutdown   = errors.New("server shutdown")
	errInvalidInvType   = errors.New("invalid inventory type")
	errInvalidHashStart = errors.New("invalid requested HashStart")
)

type (
	// Server represents the local Node in the network. Its transport could
	// be of any kind.
	Server struct {
		// ServerConfig holds the Server configuration.
		ServerConfig

		// id also known as the nonce of the server.
		id uint32

		transport Transporter
		discovery Discoverer
		chain     core.Blockchainer
		bQueue    *blockQueue
		consensus consensus.Service

		lock  sync.RWMutex
		peers map[Peer]bool

		addrReq    chan *Message
		register   chan Peer
		unregister chan peerDrop
		quit       chan struct{}

		connected *atomic.Bool
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
func NewServer(config ServerConfig, chain core.Blockchainer) *Server {
	s := &Server{
		ServerConfig: config,
		chain:        chain,
		bQueue:       newBlockQueue(maxBlockBatch, chain),
		id:           randomID(),
		quit:         make(chan struct{}),
		addrReq:      make(chan *Message, config.MinPeers),
		register:     make(chan Peer),
		unregister:   make(chan peerDrop),
		peers:        make(map[Peer]bool),
		connected:    atomic.NewBool(false),
	}

	srv, err := consensus.NewService(consensus.Config{
		Broadcast: s.handleNewPayload,
		Chain:     chain,
		RequestTx: s.requestTx,
		Wallet:    config.Wallet,
	})
	if err != nil {
		return nil
	}

	s.consensus = srv

	if s.MinPeers <= 0 {
		log.WithFields(log.Fields{
			"MinPeers configured": s.MinPeers,
			"MinPeers actual":     defaultMinPeers,
		}).Info("bad MinPeers configured, using the default value")
		s.MinPeers = defaultMinPeers
	}

	if s.MaxPeers <= 0 {
		log.WithFields(log.Fields{
			"MaxPeers configured": s.MaxPeers,
			"MaxPeers actual":     defaultMaxPeers,
		}).Info("bad MaxPeers configured, using the default value")
		s.MaxPeers = defaultMaxPeers
	}

	if s.AttemptConnPeers <= 0 {
		log.WithFields(log.Fields{
			"AttemptConnPeers configured": s.AttemptConnPeers,
			"AttemptConnPeers actual":     defaultAttemptConnPeers,
		}).Info("bad AttemptConnPeers configured, using the default value")
		s.AttemptConnPeers = defaultAttemptConnPeers
	}

	s.transport = NewTCPTransport(s, fmt.Sprintf("%s:%d", config.Address, config.Port))
	s.discovery = NewDefaultDiscovery(
		s.DialTimeout,
		s.transport,
	)

	return s
}

// ID returns the servers ID.
func (s *Server) ID() uint32 {
	return s.id
}

// Start will start the server and its underlying transport.
func (s *Server) Start(errChan chan error) {
	log.WithFields(log.Fields{
		"blockHeight":  s.chain.BlockHeight(),
		"headerHeight": s.chain.HeaderHeight(),
	}).Info("node started")

	s.discovery.BackFill(s.Seeds...)

	go s.bQueue.run()
	go s.transport.Accept()
	setServerAndNodeVersions(s.UserAgent, strconv.FormatUint(uint64(s.id), 10))
	s.run()
}

// Shutdown disconnects all peers and stops listening.
func (s *Server) Shutdown() {
	log.WithFields(log.Fields{
		"peers": s.PeerCount(),
	}).Info("shutting down server")
	s.bQueue.discard()
	close(s.quit)
}

// UnconnectedPeers returns a list of peers that are in the discovery peer list
// but are not connected to the server.
func (s *Server) UnconnectedPeers() []string {
	return []string{}
}

// BadPeers returns a list of peers the are flagged as "bad" peers.
func (s *Server) BadPeers() []string {
	return []string{}
}

func (s *Server) run() {
	for {
		if s.PeerCount() < s.MinPeers {
			s.discovery.RequestRemote(s.AttemptConnPeers)
		}
		if s.discovery.PoolCount() < minPoolCount {
			select {
			case s.addrReq <- NewMessage(s.Net, CMDGetAddr, payload.NewNullPayload()):
				// sent request
			default:
				// we have one in the queue already that is
				// gonna be served by some worker when it's ready
			}
		}
		select {
		case <-s.quit:
			s.transport.Close()
			for p := range s.peers {
				p.Disconnect(errServerShutdown)
			}
			return
		case p := <-s.register:
			s.lock.Lock()
			s.peers[p] = true
			s.lock.Unlock()
			log.WithFields(log.Fields{
				"addr": p.RemoteAddr(),
			}).Info("new peer connected")
			peerCount := s.PeerCount()
			if peerCount > s.MaxPeers {
				s.lock.RLock()
				// Pick a random peer and drop connection to it.
				for peer := range s.peers {
					peer.Disconnect(errMaxPeers)
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
				log.WithFields(log.Fields{
					"addr":      drop.peer.RemoteAddr(),
					"reason":    drop.reason,
					"peerCount": s.PeerCount(),
				}).Warn("peer disconnected")
				addr := drop.peer.PeerAddr().String()
				if drop.reason == errIdenticalID {
					s.discovery.RegisterBadAddr(addr)
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

func (s *Server) tryStartConsensus() {
	if s.Wallet == nil || s.connected.Load() {
		return
	}

	if s.PeerCount() >= s.MinPeers {
		log.Info("minimum amount of peers were connected to")
		if s.connected.CAS(false, true) {
			s.consensus.Start()
		}
	}
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

// startProtocol starts a long running background loop that interacts
// every ProtoTickInterval with the peer.
func (s *Server) startProtocol(p Peer) {
	log.WithFields(log.Fields{
		"addr":        p.RemoteAddr(),
		"userAgent":   string(p.Version().UserAgent),
		"startHeight": p.Version().StartHeight,
		"id":          p.Version().Nonce,
	}).Info("started protocol")

	s.discovery.RegisterGoodAddr(p.PeerAddr().String())
	err := s.requestHeaders(p)
	if err != nil {
		p.Disconnect(err)
		return
	}

	timer := time.NewTimer(s.ProtoTickInterval)
	for {
		select {
		case err = <-p.Done():
			// time to stop
		case m := <-s.addrReq:
			err = p.WriteMsg(m)
		case <-timer.C:
			// Try to sync in headers and block with the peer if his block height is higher then ours.
			if p.Version().StartHeight > s.chain.BlockHeight() {
				err = s.requestBlocks(p)
			}
			if err == nil {
				timer.Reset(s.ProtoTickInterval)
			}
		}
		if err != nil {
			s.unregister <- peerDrop{p, err}
			timer.Stop()
			p.Disconnect(err)
			return
		}
	}
}

// When a peer connects to the server, we will send our version immediately.
func (s *Server) sendVersion(p Peer) error {
	payload := payload.NewVersion(
		s.id,
		s.Port,
		s.UserAgent,
		s.chain.BlockHeight(),
		s.Relay,
	)
	return p.SendVersion(NewMessage(s.Net, CMDVersion, payload))
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
	peerAddr := p.PeerAddr().String()
	s.lock.RLock()
	for peer := range s.peers {
		// Already connected, drop this connection.
		if peer.Handshaked() && peer.PeerAddr().String() == peerAddr && peer.Version().Nonce == version.Nonce {
			s.lock.RUnlock()
			return errAlreadyConnected
		}
	}
	s.lock.RUnlock()
	return p.SendVersionAck(NewMessage(s.Net, CMDVerack, nil))
}

// handleHeadersCmd processes the headers received from its peer.
// If the headerHeight of the blockchain still smaller then the peer
// the server will request more headers.
// This method could best be called in a separate routine.
func (s *Server) handleHeadersCmd(p Peer, headers *payload.Headers) {
	if err := s.chain.AddHeaders(headers.Hdrs...); err != nil {
		log.Warnf("failed processing headers: %s", err)
		return
	}
	// The peer will respond with a maximum of 2000 headers in one batch.
	// We will ask one more batch here if needed. Eventually we will get synced
	// due to the startProtocol routine that will ask headers every protoTick.
	if s.chain.HeaderHeight() < p.Version().StartHeight {
		s.requestHeaders(p)
	}
}

// handleBlockCmd processes the received block received from its peer.
func (s *Server) handleBlockCmd(p Peer, block *core.Block) error {
	return s.bQueue.putBlock(block)
}

// handleInvCmd processes the received inventory.
func (s *Server) handleInvCmd(p Peer, inv *payload.Inventory) error {
	payload := payload.NewInventory(inv.Type, inv.Hashes)
	return p.WriteMsg(NewMessage(s.Net, CMDGetData, payload))
}

// handleInvCmd processes the received inventory.
func (s *Server) handleGetDataCmd(p Peer, inv *payload.Inventory) error {
	switch inv.Type {
	case payload.TXType:
		for _, hash := range inv.Hashes {
			tx, _, err := s.chain.GetTransaction(hash)
			if err == nil {
				err = p.WriteMsg(NewMessage(s.Net, CMDTX, tx))
				if err != nil {
					return err
				}

			}
		}
	case payload.BlockType:
		for _, hash := range inv.Hashes {
			b, err := s.chain.GetBlock(hash)
			if err == nil {
				err = p.WriteMsg(NewMessage(s.Net, CMDBlock, b))
				if err != nil {
					return err
				}
			}
		}
	case payload.ConsensusType:
		for _, hash := range inv.Hashes {
			if cp := s.consensus.GetPayload(hash); cp != nil {
				if err := p.WriteMsg(NewMessage(s.Net, CMDConsensus, cp)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// handleGetHeadersCmd processes the getheaders request.
func (s *Server) handleGetHeadersCmd(p Peer, gh *payload.GetBlocks) error {
	if len(gh.HashStart) < 1 {
		return errInvalidHashStart
	}
	startHash := gh.HashStart[0]
	start, err := s.chain.GetHeader(startHash)
	if err != nil {
		return err
	}
	resp := payload.Headers{}
	resp.Hdrs = make([]*core.Header, 0, payload.MaxHeadersAllowed)
	for i := start.Index + 1; i < start.Index+1+payload.MaxHeadersAllowed; i++ {
		hash := s.chain.GetHeaderHash(int(i))
		if hash.Equals(util.Uint256{}) || hash.Equals(gh.HashStop) {
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
	return p.WriteMsg(NewMessage(s.Net, CMDHeaders, &resp))
}

// handleConsensusCmd processes received consensus payload.
// It never returns an error.
func (s *Server) handleConsensusCmd(cp *consensus.Payload) error {
	s.consensus.OnPayload(cp)
	return nil
}

// handleTxCmd processes received transaction.
// It never returns an error.
func (s *Server) handleTxCmd(tx *transaction.Transaction) error {
	s.consensus.OnTransaction(tx)
	// It's OK for it to fail for various reasons like tx already existing
	// in the pool.
	_ = s.RelayTxn(tx)
	return nil
}

// handleAddrCmd will process received addresses.
func (s *Server) handleAddrCmd(p Peer, addrs *payload.AddressList) error {
	for _, a := range addrs.Addrs {
		s.discovery.BackFill(a.IPPortString())
	}
	return nil
}

// handleGetAddrCmd sends to the peer some good addresses that we know of.
func (s *Server) handleGetAddrCmd(p Peer) error {
	addrs := s.discovery.GoodPeers()
	if len(addrs) > maxAddrsToSend {
		addrs = addrs[:maxAddrsToSend]
	}
	alist := payload.NewAddressList(len(addrs))
	ts := time.Now()
	for i, addr := range addrs {
		// we know it's a good address, so it can't fail
		netaddr, _ := net.ResolveTCPAddr("tcp", addr)
		alist.Addrs[i] = payload.NewAddressAndTime(netaddr, ts)
	}
	return p.WriteMsg(NewMessage(s.Net, CMDAddr, alist))
}

// requestHeaders sends a getheaders message to the peer.
// The peer will respond with headers op to a count of 2000.
func (s *Server) requestHeaders(p Peer) error {
	start := []util.Uint256{s.chain.CurrentHeaderHash()}
	payload := payload.NewGetBlocks(start, util.Uint256{})
	return p.WriteMsg(NewMessage(s.Net, CMDGetHeaders, payload))
}

// requestBlocks sends a getdata message to the peer
// to sync up in blocks. A maximum of maxBlockBatch will
// send at once.
func (s *Server) requestBlocks(p Peer) error {
	var (
		hashes       []util.Uint256
		hashStart    = s.chain.BlockHeight() + 1
		headerHeight = s.chain.HeaderHeight()
	)
	for hashStart <= headerHeight && len(hashes) < maxBlockBatch {
		hash := s.chain.GetHeaderHash(int(hashStart))
		hashes = append(hashes, hash)
		hashStart++
	}
	if len(hashes) > 0 {
		payload := payload.NewInventory(payload.BlockType, hashes)
		return p.WriteMsg(NewMessage(s.Net, CMDGetData, payload))
	} else if s.chain.HeaderHeight() < p.Version().StartHeight {
		return s.requestHeaders(p)
	}
	return nil
}

// handleMessage processes the given message.
func (s *Server) handleMessage(peer Peer, msg *Message) error {
	// Make sure both server and peer are operating on
	// the same network.
	if msg.Magic != s.Net {
		return errInvalidNetwork
	}

	if peer.Handshaked() {
		if inv, ok := msg.Payload.(*payload.Inventory); ok {
			if !inv.Type.Valid() || len(inv.Hashes) == 0 {
				return errInvalidInvType
			}
		}
		switch msg.CommandType() {
		case CMDAddr:
			addrs := msg.Payload.(*payload.AddressList)
			return s.handleAddrCmd(peer, addrs)
		case CMDGetAddr:
			// it has no payload
			return s.handleGetAddrCmd(peer)
		case CMDGetData:
			inv := msg.Payload.(*payload.Inventory)
			return s.handleGetDataCmd(peer, inv)
		case CMDGetHeaders:
			gh := msg.Payload.(*payload.GetBlocks)
			return s.handleGetHeadersCmd(peer, gh)
		case CMDHeaders:
			headers := msg.Payload.(*payload.Headers)
			go s.handleHeadersCmd(peer, headers)
		case CMDInv:
			inventory := msg.Payload.(*payload.Inventory)
			return s.handleInvCmd(peer, inventory)
		case CMDBlock:
			block := msg.Payload.(*core.Block)
			return s.handleBlockCmd(peer, block)
		case CMDConsensus:
			cp := msg.Payload.(*consensus.Payload)
			return s.handleConsensusCmd(cp)
		case CMDTX:
			tx := msg.Payload.(*transaction.Transaction)
			return s.handleTxCmd(tx)
		case CMDVersion, CMDVerack:
			return fmt.Errorf("received '%s' after the handshake", msg.CommandType())
		}
	} else {
		switch msg.CommandType() {
		case CMDVersion:
			version := msg.Payload.(*payload.Version)
			return s.handleVersionCmd(peer, version)
		case CMDVerack:
			err := peer.HandleVersionAck()
			if err != nil {
				return err
			}
			go s.startProtocol(peer)

			s.tryStartConsensus()
		default:
			return fmt.Errorf("received '%s' during handshake", msg.CommandType())
		}
	}
	return nil
}

func (s *Server) handleNewPayload(p *consensus.Payload) {
	s.relayInventory(payload.ConsensusType, p.Hash())
}

func (s *Server) requestTx(hashes ...util.Uint256) {
	if len(hashes) == 0 {
		return
	}

	s.relayInventory(payload.TXType, hashes...)
}

func (s *Server) relayInventory(t payload.InventoryType, hashes ...util.Uint256) {
	for peer := range s.Peers() {
		if !peer.Handshaked() {
			continue
		}
		payload := payload.NewInventory(t, hashes)
		s.RelayDirectly(peer, payload)
	}
}

// RelayTxn a new transaction to the local node and the connected peers.
// Reference: the method OnRelay in C#: https://github.com/neo-project/neo/blob/master/neo/Network/P2P/LocalNode.cs#L159
func (s *Server) RelayTxn(t *transaction.Transaction) RelayReason {
	if t.Type == transaction.MinerType {
		return RelayInvalid
	}
	if s.chain.HasTransaction(t.Hash()) {
		return RelayAlreadyExists
	}
	if err := s.chain.VerifyTx(t, nil); err != nil {
		return RelayInvalid
	}
	// TODO: Implement Plugin.CheckPolicy?
	//if (!Plugin.CheckPolicy(transaction))
	// return RelayResultReason.PolicyFail;
	if ok := s.chain.GetMemPool().TryAdd(t.Hash(), core.NewPoolItem(t, s.chain)); !ok {
		return RelayOutOfMemory
	}

	for p := range s.Peers() {
		payload := payload.NewInventory(payload.TXType, []util.Uint256{t.Hash()})
		s.RelayDirectly(p, payload)
	}

	return RelaySucceed
}

// RelayDirectly relays directly the inventory to the remote peers.
// Reference: the method OnRelayDirectly in C#: https://github.com/neo-project/neo/blob/master/neo/Network/P2P/LocalNode.cs#L166
func (s *Server) RelayDirectly(p Peer, inv *payload.Inventory) {
	if !p.Version().Relay {
		return
	}

	p.WriteMsg(NewMessage(s.Net, CMDInv, inv))

}
