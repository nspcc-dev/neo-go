package network

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

const (
	maxPeers      = 10
	maxBlockBatch = 200
	minPoolCount  = 30
)

var (
	protoTickInterval = 20 * time.Second
	dialTimeout       = 3 * time.Second

	errPortMismatch     = errors.New("port mismatch")
	errIdenticalID      = errors.New("identical node id")
	errInvalidHandshake = errors.New("invalid handshake")
	errInvalidNetwork   = errors.New("invalid network")
)

// Config holds the server configuration.
type Config struct {
	// MaxPeers it the maximum numbers of peers that can
	// be connected to the server.
	MaxPeers int

	// The user agent of the server.
	UserAgent string

	// The listen address of the TCP server.
	ListenTCP uint16

	// The network mode the server will operate on.
	// ModePrivNet docker private network.
	// ModeTestNet NEO test network.
	// ModeMainNet NEO main network.
	Net NetMode

	// Relay determins whether the server is forwarding its inventory.
	Relay bool

	// Seeds are a list of initial nodes used to establish connectivity.
	Seeds []string

	// Maximum duration a single dial may take.
	DialTimeout time.Duration

	// The duration between protocol ticks with each connected peer.
	// When this is 0, the default interval of 5 seconds will be used.
	ProtoTickInterval time.Duration

	// Level of the internal logger.
	LogLevel log.Level
}

type (
	// Server represents the local Node in the network. Its transport could
	// be of any kind.
	Server struct {
		// Config holds the Server configuration.
		Config

		// id also known as the nonce of te server.
		id uint32

		transport Transporter
		discovery Discoverer
		chain     core.Blockchainer

		lock  sync.RWMutex
		peers map[Peer]bool

		register   chan Peer
		unregister chan peerDrop
		quit       chan struct{}

		proto <-chan protoTuple
	}

	protoTuple struct {
		msg  *Message
		peer Peer
	}

	peerDrop struct {
		peer   Peer
		reason error
	}
)

// NewServer returns a new Server, initialized with the given configuration.
func NewServer(cfg Config, chain *core.Blockchain) *Server {
	if cfg.ProtoTickInterval == 0 {
		cfg.ProtoTickInterval = protoTickInterval
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = dialTimeout
	}
	if cfg.MaxPeers == 0 {
		cfg.MaxPeers = maxPeers
	}
	log.SetLevel(log.DebugLevel)

	s := &Server{
		Config:     cfg,
		chain:      chain,
		id:         util.RandUint32(1000000, 9999999),
		quit:       make(chan struct{}),
		register:   make(chan Peer),
		unregister: make(chan peerDrop),
		peers:      make(map[Peer]bool),
	}

	s.transport = NewTCPTransport(s, fmt.Sprintf(":%d", cfg.ListenTCP))
	s.proto = s.transport.Consumer()
	s.discovery = NewDefaultDiscovery(
		s.DialTimeout,
		s.transport,
	)

	return s
}

// Start will start the server and its underlying transport.
func (s *Server) Start() {
	go s.transport.Accept()
	s.discovery.BackFill(s.Seeds...)
	s.run()
}

func (s *Server) run() {
	// As discovery to connect with remote nodes.
	n := s.MaxPeers - s.PeerCount()
	s.discovery.RequestRemote(n)

	for {
		select {
		case proto := <-s.proto:
			if err := s.processProto(proto); err != nil {
				proto.peer.Disconnect(err)
			}
		case <-s.quit:
			s.transport.Close()
			for p, _ := range s.peers {
				p.Disconnect(errors.New("server shutdown"))
			}
			return
		case p := <-s.register:
			s.peers[p] = true
			// When a new peer is connected we send out our version immediately.
			s.sendVersion(p)
			log.WithFields(log.Fields{
				"endpoint": p.Endpoint(),
			}).Info("new peer connected")
		case drop := <-s.unregister:
			s.discovery.RequestRemote(1)
			delete(s.peers, drop.peer)
			panic("kekdjkf")
			log.WithFields(log.Fields{
				"endpoint":  drop.peer.Endpoint(),
				"reason":    drop.reason,
				"peerCount": s.PeerCount(),
			}).Info("peer disconnected")
		}
	}
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
		"endpoint":    p.Endpoint(),
		"userAgent":   string(p.Version().UserAgent),
		"startHeight": p.Version().StartHeight,
		"id":          p.Version().Nonce,
	}).Info("started protocol")

	go s.requestHeaders(p)
	s.requestPeerInfo(p)

	timer := time.NewTimer(s.ProtoTickInterval)
	for {
		select {
		case <-p.Done():
			return
		case <-timer.C:
			// Try to sync in headers and block with the peer if his block height is higher then ours.
			if p.Version().StartHeight > s.chain.BlockHeight() {
				s.requestBlocks(p)
			}

			// If the discovery does not have a healthy address pool
			// we will ask for a new batch of addresses.
			if s.discovery.PoolCount() < minPoolCount {
				s.requestPeerInfo(p)
			}

			timer.Reset(s.ProtoTickInterval)
		}
	}
}

// When a peer connects to the server, we will send our version immediately.
func (s *Server) sendVersion(p Peer) {
	payload := payload.NewVersion(s.id, s.ListenTCP, s.UserAgent, s.chain.BlockHeight(), s.Relay)
	p.Send(NewMessage(s.Net, CMDVersion, payload))
}

// When a peer sends out his version we reply with verack after validating
// the version.
func (s *Server) handleVersionCmd(p Peer, version *payload.Version) error {
	if p.Endpoint().Port != version.Port {
		return errPortMismatch
	}
	if s.id == version.Nonce {
		return errIdenticalID
	}
	return p.Send(NewMessage(s.Net, CMDVerack, nil))
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
		go s.requestHeaders(p)
	}
}

// handleBlockCmd processes the received block received from its peer.
func (s *Server) handleBlockCmd(p Peer, block *core.Block) {
	if err := s.chain.AddBlock(block); err != nil {
		log.Warnf("failed to process block: %s", err)
	}
}

// handleInvCmd will process the received inventory.
func (s *Server) handleInvCmd(p Peer, inv *payload.Inventory) {
	if !inv.Type.Valid() || len(inv.Hashes) == 0 {
		return
	}
	// log.Debugf("received inventory %s", inv.Type)
	payload := payload.NewInventory(inv.Type, inv.Hashes)
	p.Send(NewMessage(s.Net, CMDGetData, payload))
}

func (s *Server) handleGetHeadersCmd(p Peer, getHeaders *payload.GetBlocks) {
	log.Info(getHeaders)
}

// requestHeaders will send a getheaders message to the peer.
// The peer will respond with headers op to a count of 2000.
func (s *Server) requestHeaders(p Peer) {
	start := []util.Uint256{s.chain.CurrentHeaderHash()}
	payload := payload.NewGetBlocks(start, util.Uint256{})
	p.Send(NewMessage(s.Net, CMDGetHeaders, payload))
}

// requestPeerInfo will send a getaddr message to the peer
// which will respond with his known addresses in the network.
func (s *Server) requestPeerInfo(p Peer) {
	p.Send(NewMessage(s.Net, CMDGetAddr, nil))
}

// requestBlocks will send a getdata message to the peer
// to sync up in blocks. A maximum of maxBlockBatch will
// send at once.
func (s *Server) requestBlocks(p Peer) {
	var (
		hashStart    = s.chain.BlockHeight() + 1
		headerHeight = s.chain.HeaderHeight()
		hashes       = []util.Uint256{}
	)
	for hashStart < headerHeight && len(hashes) < maxBlockBatch {
		hash := s.chain.GetHeaderHash(int(hashStart))
		hashes = append(hashes, hash)
		hashStart++
	}
	if len(hashes) > 0 {
		payload := payload.NewInventory(payload.BlockType, hashes)
		p.Send(NewMessage(s.Net, CMDGetData, payload))
	} else if s.chain.HeaderHeight() < p.Version().StartHeight {
		s.requestHeaders(p)
	}
}

// process the received protocol message.
func (s *Server) processProto(proto protoTuple) error {
	var (
		peer = proto.peer
		msg  = proto.msg
	)

	// Make sure both server and peer are operating on
	// the same network.
	if msg.Magic != s.Net {
		return errInvalidNetwork
	}

	switch msg.CommandType() {
	case CMDVersion:
		version := msg.Payload.(*payload.Version)
		return s.handleVersionCmd(peer, version)
	case CMDHeaders:
		headers := msg.Payload.(*payload.Headers)
		s.handleHeadersCmd(peer, headers)
	case CMDInv:
		inventory := msg.Payload.(*payload.Inventory)
		s.handleInvCmd(peer, inventory)
	case CMDBlock:
		block := msg.Payload.(*core.Block)
		s.handleBlockCmd(peer, block)
	case CMDGetHeaders:
		getHeaders := msg.Payload.(*payload.GetBlocks)
		s.handleGetHeadersCmd(peer, getHeaders)
	case CMDVerack:
		// Make sure this peer has sended his version before we start the
		// protocol.
		if peer.Version() == nil {
			return errInvalidHandshake
		}
		go s.startProtocol(peer)
	case CMDAddr:
		addressList := msg.Payload.(*payload.AddressList)
		for _, addr := range addressList.Addrs {
			s.discovery.BackFill(addr.Endpoint.String())
		}
	}
	return nil
}
