package network

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

const (
	// peer numbers are arbitrary at the moment
	minPeers      = 5
	maxPeers      = 20
	maxBlockBatch = 200
	minPoolCount  = 30
)

var (
	errPortMismatch     = errors.New("port mismatch")
	errIdenticalID      = errors.New("identical node id")
	errInvalidHandshake = errors.New("invalid handshake")
	errInvalidNetwork   = errors.New("invalid network")
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

		transport Transporter
		discovery Discoverer
		chain     core.Blockchainer

		lock  sync.RWMutex
		peers map[Peer]bool

		addrReq    chan *Message
		register   chan Peer
		unregister chan peerDrop
		quit       chan struct{}
	}

	peerDrop struct {
		peer   Peer
		reason error
	}
)

// NewServer returns a new Server, initialized with the given configuration.
func NewServer(config ServerConfig, chain core.Blockchainer) *Server {
	s := &Server{
		ServerConfig: config,
		chain:        chain,
		id:           rand.Uint32(),
		quit:         make(chan struct{}),
		addrReq:      make(chan *Message, minPeers),
		register:     make(chan Peer),
		unregister:   make(chan peerDrop),
		peers:        make(map[Peer]bool),
	}

	s.transport = NewTCPTransport(s, fmt.Sprintf(":%d", config.ListenTCP))
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

	go s.transport.Accept()
	s.run()
}

// Shutdown disconnects all peers and stops listening.
func (s *Server) Shutdown() {
	log.WithFields(log.Fields{
		"peers": s.PeerCount(),
	}).Info("shutting down server")
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
		c := s.PeerCount()
		if c < minPeers {
			s.discovery.RequestRemote(maxPeers - c)
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
			// When a new peer is connected we send out our version immediately.
			if err := s.sendVersion(p); err != nil {
				log.WithFields(log.Fields{
					"addr": p.NetAddr(),
				}).Error(err)
			}
			s.peers[p] = true
			log.WithFields(log.Fields{
				"addr": p.NetAddr(),
			}).Info("new peer connected")
		case drop := <-s.unregister:
			if s.peers[drop.peer] {
				delete(s.peers, drop.peer)
				log.WithFields(log.Fields{
					"addr":      drop.peer.NetAddr(),
					"reason":    drop.reason,
					"peerCount": s.PeerCount(),
				}).Warn("peer disconnected")
				addr := drop.peer.NetAddr().String()
				s.discovery.UnregisterConnectedAddr(addr)
				s.discovery.BackFill(addr)
			}
			// else the peer is already gone, which can happen
			// because we have two goroutines sending signals here
		}
	}
}

// Peers returns the current list of peers connected to
// the server.
func (s *Server) Peers() map[Peer]bool {
	return s.peers
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
		"addr":        p.NetAddr(),
		"userAgent":   string(p.Version().UserAgent),
		"startHeight": p.Version().StartHeight,
		"id":          p.Version().Nonce,
	}).Info("started protocol")

	s.discovery.RegisterGoodAddr(p.NetAddr().String())
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
		s.ListenTCP,
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
	if p.NetAddr().Port != int(version.Port) {
		return errPortMismatch
	}
	return p.SendVersionAck(NewMessage(s.Net, CMDVerack, nil))
}

// handleHeadersCmd will process the headers it received from its peer.
// if the headerHeight of the blockchain still smaller then the peer
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
	if !s.chain.HasBlock(block.Hash()) {
		return s.chain.AddBlock(block)
	}
	return nil
}

// handleInvCmd will process the received inventory.
func (s *Server) handleInvCmd(p Peer, inv *payload.Inventory) error {
	if !inv.Type.Valid() || len(inv.Hashes) == 0 {
		return errInvalidInvType
	}
	payload := payload.NewInventory(inv.Type, inv.Hashes)
	return p.WriteMsg(NewMessage(s.Net, CMDGetData, payload))
}

// handleAddrCmd will process received addresses.
func (s *Server) handleAddrCmd(p Peer, addrs *payload.AddressList) error {
	for _, a := range addrs.Addrs {
		s.discovery.BackFill(a.IPPortString())
	}
	return nil
}

// requestHeaders will send a getheaders message to the peer.
// The peer will respond with headers op to a count of 2000.
func (s *Server) requestHeaders(p Peer) error {
	start := []util.Uint256{s.chain.CurrentHeaderHash()}
	payload := payload.NewGetBlocks(start, util.Uint256{})
	return p.WriteMsg(NewMessage(s.Net, CMDGetHeaders, payload))
}

// requestBlocks will send a getdata message to the peer
// to sync up in blocks. A maximum of maxBlockBatch will
// send at once.
func (s *Server) requestBlocks(p Peer) error {
	var (
		hashes       []util.Uint256
		hashStart    = s.chain.BlockHeight() + 1
		headerHeight = s.chain.HeaderHeight()
	)
	for hashStart < headerHeight && len(hashes) < maxBlockBatch {
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

// handleMessage will process the given message.
func (s *Server) handleMessage(peer Peer, msg *Message) error {
	// Make sure both server and peer are operating on
	// the same network.
	if msg.Magic != s.Net {
		return errInvalidNetwork
	}

	if peer.Handshaked() {
		switch msg.CommandType() {
		case CMDAddr:
			addrs := msg.Payload.(*payload.AddressList)
			return s.handleAddrCmd(peer, addrs)
		case CMDHeaders:
			headers := msg.Payload.(*payload.Headers)
			go s.handleHeadersCmd(peer, headers)
		case CMDInv:
			inventory := msg.Payload.(*payload.Inventory)
			return s.handleInvCmd(peer, inventory)
		case CMDBlock:
			block := msg.Payload.(*core.Block)
			return s.handleBlockCmd(peer, block)
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
		default:
			return fmt.Errorf("received '%s' during handshake", msg.CommandType())
		}
	}
	return nil
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
	if err := s.chain.Verify(t); err != nil {
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

// RelayDirectly relay directly the inventory to the remote peers.
// Reference: the method OnRelayDirectly in C#: https://github.com/neo-project/neo/blob/master/neo/Network/P2P/LocalNode.cs#L166
func (s *Server) RelayDirectly(p Peer, inv *payload.Inventory) {
	if !p.Version().Relay {
		return
	}

	p.WriteMsg(NewMessage(s.Net, CMDInv, inv))

}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
