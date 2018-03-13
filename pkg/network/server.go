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
	maxPeers           = 4
	healthyPoolCount   = 50
	maxBlockBatchCount = 200
	minPoolCount       = 10
)

var (
	protoTickInterval = 20 * time.Second
	dialTimeout       = 3 * time.Second
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
		chain     *core.Blockchain

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

	ts := NewTCPTransport(fmt.Sprintf(":%d", cfg.ListenTCP))
	s := &Server{
		Config:     cfg,
		chain:      chain,
		id:         util.RandUint32(1000000, 9999999),
		quit:       make(chan struct{}),
		register:   make(chan Peer),
		unregister: make(chan peerDrop),
		peers:      make(map[Peer]bool),
		proto:      ts.Consumer(),
		transport:  ts,
	}

	s.discovery = NewDefaultDiscovery(
		s.DialTimeout,
		s.transport,
	)

	return s
}

// Start will start the server and its underlying transport.
func (s *Server) Start() {
	go s.run()
	go s.transport.Accept(s)
	go s.connectWithSeeds(s.Seeds...)
	select {}
}

func (s *Server) run() {
	// As discovery to connect with remote nodes.
	n := s.MaxPeers - s.PeerCount()
	s.discovery.Request(n)

	for {
		select {
		case proto := <-s.proto:
			s.processProto(proto)
		case <-s.quit:
			s.transport.Close()
			for p, _ := range s.peers {
				p.Disconnect(errors.New("server shutdown"))
			}
			return
		case p := <-s.register:
			if len(s.peers) == s.MaxPeers {
				break
			}
			s.peers[p] = true
			// When a new peer is connected we send out our version immediately.
			s.sendVersion(p)
			log.Debugf("peer connected %s", p.Endpoint())
		case drop := <-s.unregister:
			delete(s.peers, drop.peer)
			log.WithFields(log.Fields{
				"endpoint":  drop.peer.Endpoint(),
				"reason":    drop.reason,
				"peerCount": s.PeerCount(),
			}).Debug("peer disconnected")
		}
	}
}

// PeerCount returns the number of current connected peers.
func (s *Server) PeerCount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.peers)
}

func (s *Server) connectWithSeeds(addrs ...string) {
	for _, addr := range addrs {
		go func(addr string) {
			s.transport.Dial(addr, s.DialTimeout)
		}(addr)
	}
}

// startProtocol starts a long running background loop that interacts
// every ProtoTickInterval with the peer.
func (s *Server) startProtocol(p Peer) {
	s.askMoreHeaders(p)

	timer := time.NewTimer(s.ProtoTickInterval)
	for {
		select {
		case <-p.Done():
			return
		case <-timer.C:
			// Try to sync in headers and block with the peer if his block height is higher then ours.
			if p.Version().StartHeight > s.chain.HeaderHeight() {
				s.askMoreBlocks(p)
			}

			// If the discovery does not have a healthy address pool
			// we will ask for a new batch of addresses.
			if s.discovery.PoolCount() < minPoolCount {
				p.Send(NewMessage(s.Net, CMDGetAddr, nil))
			}

			timer.Reset(s.ProtoTickInterval)
		}
	}
}

// When a peer connects to the server, we will send our version immediately.
func (s *Server) sendVersion(p Peer) {
	payload := payload.NewVersion(s.id, s.ListenTCP, s.UserAgent, 0, s.Relay)
	p.Send(NewMessage(s.Net, CMDVersion, payload))
}

// When a peer sends out his version we reply with verack after validating
// the version.
func (s *Server) handleVersionCmd(p Peer, version *payload.Version) {
	if p.Endpoint().Port != version.Port {
		p.Disconnect(errors.New("port mismatch"))
		return
	}
	if s.id == version.Nonce {
		p.Disconnect(errors.New("identical node id"))
		return
	}
	p.Send(NewMessage(s.Net, CMDVerack, nil))
}

// The handleHeadersCmd will process the received headers from its peer.
func (s *Server) handleHeadersCmd(p Peer, headers *payload.Headers) {
	go func(headers []*core.Header) {
		if err := s.chain.AddHeaders(headers...); err != nil {
			log.Debug(err)
			return
		}
		// The peer will respond with a maximum of 2000 headers in one batch.
		// We will ask one more batch here if needed. Eventually we will get synced
		// due to the startProtocol routine that will ask headers every protoTick.
		if s.chain.HeaderHeight() < p.Version().StartHeight {
			s.askMoreHeaders(p)
		}
	}(headers.Hdrs)
}

// handleBlockCmd processes the received block received from its peer.
func (s *Server) handleBlockCmd(p Peer, block *core.Block) {
	if err := s.chain.AddBlock(block); err != nil {
		log.Debug(err)
	}
}

func (s *Server) handleInvCmd(p Peer, inv *payload.Inventory) {
	if !inv.Type.Valid() || len(inv.Hashes) == 0 {
		return
	}
	log.Debugf("received inventory %s", inv.Type)
	payload := payload.NewInventory(inv.Type, inv.Hashes)
	p.Send(NewMessage(s.Net, CMDGetData, payload))
}

// askMoreHeaders will send a getheaders message to the peer.
func (s *Server) askMoreHeaders(p Peer) {
	start := []util.Uint256{s.chain.CurrentHeaderHash()}
	payload := payload.NewGetBlocks(start, util.Uint256{})
	p.Send(NewMessage(s.Net, CMDGetHeaders, payload))
}

// askMoreBlocks will send a getdata message to the peer
// to sync up in blocks.
func (s *Server) askMoreBlocks(p Peer) {
	var (
		hashStart    = s.chain.BlockHeight() + 1
		headerHeight = s.chain.HeaderHeight()
		hashes       = []util.Uint256{}
	)
	for hashStart < headerHeight && len(hashes) < maxBlockBatchCount {
		hash := s.chain.GetHeaderHash(int(hashStart))
		hashes = append(hashes, hash)
		hashStart++
	}
	if len(hashes) > 0 {
		payload := payload.NewInventory(payload.BlockType, hashes)
		p.Send(NewMessage(s.Net, CMDGetData, payload))
	} else if s.chain.HeaderHeight() < p.Version().StartHeight {
		s.askMoreHeaders(p)
	}
}

// process the received protocol message.
func (s *Server) processProto(proto protoTuple) {
	var (
		peer = proto.peer
		msg  = proto.msg
	)

	// Make sure both server and peer are operating on
	// the same network.
	if msg.Magic != s.Net {
		peer.Disconnect(errors.New("invalid network"))
		return
	}

	switch msg.CommandType() {
	case CMDVersion:
		version := msg.Payload.(*payload.Version)
		s.handleVersionCmd(peer, version)
	case CMDHeaders:
		headers := msg.Payload.(*payload.Headers)
		s.handleHeadersCmd(peer, headers)
	case CMDInv:
		inventory := msg.Payload.(*payload.Inventory)
		s.handleInvCmd(peer, inventory)
	case CMDBlock:
		block := msg.Payload.(*core.Block)
		s.handleBlockCmd(peer, block)
	case CMDVerack:
		// Make sure this peer has sended his version before we start the
		// protocol.
		if peer.Version() == nil {
			peer.Disconnect(errors.New("invalid handshake"))
			return
		}
		go s.startProtocol(peer)
	case CMDAddr:
		addressList := msg.Payload.(*payload.AddressList)
		go s.discovery.BackFill(addressList)
	}
}
