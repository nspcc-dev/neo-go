package p2p

import (
	"os"

	log "github.com/go-kit/kit/log"
)

type Server struct {
	logger    log.Logger
	transport Transporter

	peerOp     chan peerOpFunc
	peerOpDone chan struct{}

	register   chan Peer
	unregister chan peerDrop
	quit       chan struct{}
}

type peerDrop struct {
	peer   Peer
	reason error
}

type peerOpFunc func(peers map[Peer]bool)

func NewServer() *Server {
	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	l = log.With(l, "component", "server")
	s := &Server{
		quit:       make(chan struct{}),
		peerOp:     make(chan peerOpFunc),
		peerOpDone: make(chan struct{}),
		register:   make(chan Peer),
		unregister: make(chan peerDrop),
		transport:  NewTCPTransport(":3002"),
		logger:     l,
	}
	return s
}

// Start will start the server and its underlying transport.
func (s *Server) Start() {
	go s.transport.Accept(s)
	s.run()
}

func (s *Server) run() {
	var (
		peers = make(map[Peer]bool)
		//badAddrs map[string]bool
	)

	for {
		select {
		case <-s.quit:
			s.transport.Close()
			for p, _ := range peers {
				p.Disconnect()
			}
			return
		case op := <-s.peerOp:
			op(peers)
			s.peerOpDone <- struct{}{}
		case p := <-s.register:
			peers[p] = true
			s.logger.Log("event", "peer connected", "endpoint", p.Endpoint())
		case drop := <-s.unregister:
			delete(peers, drop.peer)
			s.logger.Log(
				"event", "peer disconnected",
				"endpoint", drop.peer.Endpoint(),
				"reason", drop.reason,
			)
		}
	}
}
