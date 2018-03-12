package p2p

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/anthdm/neo-go/pkg/util"
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

		lock     sync.RWMutex
		peers    map[Peer]bool
		badAddrs map[string]bool

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
	ts := NewTCPTransport(fmt.Sprintf(":%d", cfg.ListenTCP))
	s := &Server{
		Config:     cfg,
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
			s.sendVersion(p)
			s.logger.Log("event", "peer connected", "endpoint", p.Endpoint())
		case drop := <-s.unregister:
			delete(s.peers, drop.peer)
			s.logger.Log(
				"event", "peer disconnected",
				"endpoint", drop.peer.Endpoint(),
				"reason", drop.reason,
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
			if err := s.transport.Dial(addr, s.DialTimeout); err != nil {
			}
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
			// If the discovery is not healthy we will ask for a new batch
			// of addresses.
			if !s.discovery.Healthy() {
				p.Send(NewMessage(s.Net, CMDGetAddr, nil))
			}
			timer.Reset(s.ProtoTickInterval)
		}
	}
}

// When a peer connects to the server, we will send our version immediately.
func (s *Server) sendVersion(p Peer) {
	payload := payload.NewVersion(s.id, 10333, s.UserAgent, 0, s.Relay)
	p.Send(NewMessage(s.Net, CMDVersion, payload))
}

// When a peer sends out his version we reply with verack after validating
// the version.
func (s *Server) handleVersionCmd(p Peer, version *payload.Version) {
	p.Send(NewMessage(s.Net, CMDVerack, nil))
}

// process the received protocol message.
func (s *Server) processProto(proto protoTuple) {
	var (
		peer = proto.peer
		msg  = proto.msg
	)

	switch msg.CommandType() {
	case CMDVersion:
		version := msg.Payload.(*payload.Version)
		s.handleVersionCmd(peer, version)
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
