package p2p

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/go-kit/kit/log"
)

const (
	maxPeers         = 2
	healthyPoolCount = 50
)

var (
	protoTickInterval   = 5 * time.Second
	errInvalidHandshake = errors.New("peer sended verack before version")
	dialTimeout         = 3 * time.Second
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
}

type (
	// Server represents the local Node in the network. Its transport could
	// be of any kind.
	Server struct {
		// Config holds the Server configuration.
		Config

		// id also known as the nonce of te server.
		id uint32

		logger    log.Logger
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
func NewServer(cfg Config) *Server {
	if cfg.ProtoTickInterval == 0 {
		cfg.ProtoTickInterval = protoTickInterval
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = dialTimeout
	}

	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	l = log.With(l, "component", "server")

	chain, err := setupBlockchain(cfg.Net)
	if err != nil {
		l.Log(err)
		os.Exit(0)
	}

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
		logger:     l,
	}

	discCfg := discoveryConfig{
		maxPeers:         s.MaxPeers,
		dialTimeout:      s.DialTimeout,
		healthyPoolCount: 100,
		transport:        s.transport,
		peerCount:        s.PeerCount,
	}
	s.discovery = NewDefaultDiscovery(discCfg)

	return s
}

func setupBlockchain(net NetMode) (*core.Blockchain, error) {
	var startHash util.Uint256
	if net == ModePrivNet {
		startHash = core.GenesisHashPrivNet()
	}
	if net == ModeTestNet {
		startHash = core.GenesisHashTestNet()
	}
	if net == ModeMainNet {
		startHash = core.GenesisHashMainNet()
	}

	// Hardcoded for now.
	store, err := core.NewLevelDBStore("chain", nil)
	if err != nil {
		return nil, err
	}

	return core.NewBlockchain(
		store,
		startHash,
	), nil
}

// Start will start the server and its underlying transport.
func (s *Server) Start() {
	go s.run()
	go s.transport.Accept(s)
	go s.connectWithSeeds(s.Seeds...)
	select {}
}

func (s *Server) run() {
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
			// When a new peer is connected we send our version immediately.
			s.sendVersion(p)
			s.logger.Log("event", "peer connected", "endpoint", p.Endpoint())
		case drop := <-s.unregister:
			delete(s.peers, drop.peer)
			s.logger.Log(
				"event", "peer disconnected",
				"endpoint", drop.peer.Endpoint(),
				"reason", drop.reason,
				"peer_count", s.PeerCount(),
			)
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
	timer := time.NewTimer(s.ProtoTickInterval)
	for {
		select {
		case <-p.Done():
			return
		case <-timer.C:
			if _, ok := s.peers[p]; !ok {
				return
			}

			// Try to sync in headers with the peer if his block height is higher then ours.
			if p.Version().StartHeight > s.chain.HeaderHeight() {
				go s.askMoreHeaders(p)
			}

			// If the discovery does not have a healthy address pool
			// we will ask for a new batch of addresses.
			if !s.discovery.Healthy() {
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
			s.logger.Log("msg", "failed processing headers", "err", err)
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

// askMoreHeaders will send a getheaders message to the peer.
func (s *Server) askMoreHeaders(p Peer) {
	start := []util.Uint256{s.chain.CurrentHeaderHash()}
	payload := payload.NewGetBlocks(start, util.Uint256{})
	p.Send(NewMessage(s.Net, CMDGetHeaders, payload))
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
	case CMDBlock:
	case CMDVerack:
		// Make sure this peer has sended his version before we start the
		// protocol.
		if peer.Version() == nil {
			peer.Disconnect(errInvalidHandshake)
			return
		}
		go s.startProtocol(peer)
	case CMDAddr:
		addressList := msg.Payload.(*payload.AddressList)
		go s.discovery.BackFill(addressList)
	}
}
