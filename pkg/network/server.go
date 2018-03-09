package network

import (
	"fmt"
	"net"
	"os"
	"text/tabwriter"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/go-kit/kit/log"
)

const (
	// node version
	version = "2.6.0"

	// official ports according to the protocol.
	portMainNet = 10333
	portTestNet = 20333
	maxPeers    = 10
)

var dialTimeout = 4 * time.Second

// Config holds the server configuration.
type Config struct {
	// MaxPeers it the maximum numbers of peers that can
	// be connected to the server.
	MaxPeers int

	// The user agent of the server.
	UserAgent string

	// The listen address of the TCP server.
	ListenTCP uint16

	// The listen address of the RPC server.
	ListenRPC uint16

	// The network mode this server will operate on.
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
}

// Server manages all incoming peer connections.
type Server struct {
	// Config fields may not be modified while the server is running.
	Config

	// Proto is just about anything that can handle the NEO protocol.
	// In production enviroments the ProtoHandler is mostly the local node.
	proto ProtoHandler

	// Unique id of this server.
	id uint32

	logger   log.Logger
	listener net.Listener

	register   chan Peer
	unregister chan Peer

	badAddrOp     chan func(map[string]bool)
	badAddrOpDone chan struct{}

	peerOp     chan func(map[Peer]bool)
	peerOpDone chan struct{}

	quit chan struct{}
}

// NewServer returns a new Server object created from the
// given config.
func NewServer(cfg Config) *Server {
	if cfg.MaxPeers == 0 {
		cfg.MaxPeers = maxPeers
	}
	if cfg.Net == 0 {
		cfg.Net = ModeTestNet
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = dialTimeout
	}

	logger := log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "component", "server")

	s := &Server{
		Config:        cfg,
		logger:        logger,
		id:            util.RandUint32(1000000, 9999999),
		quit:          make(chan struct{}, 1),
		register:      make(chan Peer),
		unregister:    make(chan Peer),
		badAddrOp:     make(chan func(map[string]bool)),
		badAddrOpDone: make(chan struct{}),
		peerOp:        make(chan func(map[Peer]bool)),
		peerOpDone:    make(chan struct{}),
	}

	s.proto = newNode(s, cfg)

	return s
}

func (s *Server) createListener() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.ListenTCP))
	if err != nil {
		return err
	}
	s.listener = ln
	return nil
}

func (s *Server) listenTCP() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.logger.Log("msg", "conn read error", "err", err)
			break
		}
		go s.setupConnection(conn)
	}
	s.Quit()
}

func (s *Server) setupConnection(conn net.Conn) {
	if !s.hasCapacity() {
		s.logger.Log("msg", "server reached maximum capacity")
		return
	}

	p := NewTCPPeer(conn, s.proto.handleProto)
	s.register <- p
	if err := p.run(); err != nil {
		s.unregister <- p
	}
}

func (s *Server) connectToPeers(addrs ...string) {
	for _, addr := range addrs {
		if s.hasCapacity() && s.canConnectWith(addr) {
			go func(addr string) {
				conn, err := net.DialTimeout("tcp", addr, s.DialTimeout)
				if err != nil {
					s.badAddrOp <- func(badAddrs map[string]bool) {
						badAddrs[addr] = true
					}
					<-s.badAddrOpDone
					return
				}
				go s.setupConnection(conn)
			}(addr)
		}
	}
}

func (s *Server) canConnectWith(addr string) bool {
	canConnect := true
	s.peerOp <- func(peers map[Peer]bool) {
		for peer := range peers {
			if peer.Endpoint().String() == addr {
				canConnect = false
				break
			}
		}
	}
	<-s.peerOpDone
	if !canConnect {
		return false
	}

	s.badAddrOp <- func(badAddrs map[string]bool) {
		_, ok := badAddrs[addr]
		canConnect = !ok
	}
	<-s.badAddrOpDone
	return canConnect
}

func (s *Server) hasCapacity() bool {
	return s.PeerCount() != s.MaxPeers
}

func (s *Server) sendVersion(p Peer) {
	p.Send(NewMessage(s.Net, CMDVersion, s.proto.version()))
}

func (s *Server) run() {
	var (
		peers    = make(map[Peer]bool)
		badAddrs = make(map[string]bool)
	)

	for {
		select {
		case op := <-s.badAddrOp:
			op(badAddrs)
			s.badAddrOpDone <- struct{}{}
		case op := <-s.peerOp:
			op(peers)
			s.peerOpDone <- struct{}{}
		case p := <-s.register:
			peers[p] = true
			// When a new peer connection is established, we send
			// out our version immediately.
			s.sendVersion(p)
			s.logger.Log("event", "peer connected", "endpoint", p.Endpoint())
		case p := <-s.unregister:
			delete(peers, p)
			s.logger.Log("event", "peer disconnected", "endpoint", p.Endpoint())
		case <-s.quit:
			return
		}
	}
}

// PeerCount returns the number of current connected peers.
func (s *Server) PeerCount() (n int) {
	s.peerOp <- func(peers map[Peer]bool) {
		n = len(peers)
	}
	<-s.peerOpDone
	return
}

func (s *Server) Start() error {
	fmt.Println(logo())
	fmt.Println("")
	s.printConfiguration()

	if err := s.createListener(); err != nil {
		return err
	}

	go s.run()
	go s.listenTCP()
	go s.connectToPeers(s.Seeds...)
	select {}
}

func (s *Server) Quit() {
	s.quit <- struct{}{}
}

func (s *Server) printState() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintf(w, "connected peers:\t%d/%d\n", s.PeerCount(), s.MaxPeers)
	w.Flush()
}

func (s *Server) printConfiguration() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintf(w, "user agent:\t%s\n", s.UserAgent)
	fmt.Fprintf(w, "id:\t%d\n", s.id)
	fmt.Fprintf(w, "network:\t%s\n", s.Net)
	fmt.Fprintf(w, "listen TCP:\t%d\n", s.ListenTCP)
	fmt.Fprintf(w, "listen RPC:\t%d\n", s.ListenRPC)
	fmt.Fprintf(w, "relay:\t%v\n", s.Relay)
	fmt.Fprintf(w, "max peers:\t%d\n", s.MaxPeers)
	chainer := s.proto.(Noder)
	fmt.Fprintf(w, "current height:\t%d\n", chainer.blockchain().HeaderHeight())
	fmt.Fprintln(w, "")
	w.Flush()
}

func logo() string {
	return `
    _   ____________        __________
   / | / / ____/ __ \      / ____/ __ \
  /  |/ / __/ / / / /_____/ / __/ / / /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /
/_/ |_/_____/\____/      \____/\____/
`
}
