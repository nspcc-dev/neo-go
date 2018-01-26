package network

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
)

const (
	version     = "0.0.1"
	portMainNet = 10333
	portTestNet = 20333
	// make sure we can run a server without consuming
	// docker privnet ports.
	portDevNet = 3000
)

type messageTuple struct {
	peer *Peer
	msg  *Message
}

// Server is the representation of a full working NEO TCP node.
type Server struct {
	logger *log.Logger

	// userAgent of the server.
	userAgent string
	// The "magic" mode the server is currently running on.
	// This can either be 0x00746e41 or 0x74746e41 for main or test net.
	netMode uint32
	// map that holds all connected peers to this server.
	peers map[*Peer]bool

	register   chan *Peer
	unregister chan *Peer

	// channel for coordinating messages.
	message chan messageTuple

	// channel used to gracefull shutdown the server.
	quit chan struct{}

	// Whether this server will receive and forward messages.
	relay bool

	// TCP listener of the server
	listener net.Listener

	dev bool
}

// NewServer returns a pointer to a new server.
func NewServer(mode uint32) *Server {
	logger := log.New(os.Stdout, "NEO SERVER :: ", 0)

	if mode != ModeTestNet && mode != ModeMainNet {
		logger.Fatalf("invalid network mode %d", mode)
	}

	s := &Server{
		userAgent:  fmt.Sprintf("/NEO:%s/", version),
		logger:     logger,
		peers:      make(map[*Peer]bool),
		register:   make(chan *Peer),
		unregister: make(chan *Peer),
		message:    make(chan messageTuple),
		relay:      true,
		netMode:    mode,
		quit:       make(chan struct{}),
	}

	return s
}

// Start run's the server.
func (s *Server) Start(port string, seeds []string) {
	fmt.Println(logo())
	s.logger.Printf("running %s on %s - relay: %v", s.userAgent, "testnet", s.relay)

	go listenTCP(s, port)

	if len(seeds) > 0 {
		connectToSeeds(s, seeds)
	}

	s.loop()
}

// Stop the server, attemping a gracefull shutdown.
func (s *Server) Stop() { s.quit <- struct{}{} }

// shutdown the server, disconnecting all peers.
func (s *Server) shutdown() {
	s.logger.Println("attemping a quitefull shutdown.")
	s.listener.Close()

	// disconnect and remove all connected peers.
	for peer := range s.peers {
		peer.conn.Close()
		s.unregister <- peer
	}
}

func (s *Server) loop() {
	for {
		select {
		case peer := <-s.register:
			s.logger.Printf("peer registered from address %s", peer.conn.RemoteAddr())
			resp, err := s.handlePeerConnected()
			if err != nil {
				s.logger.Fatalf("handling initial peer connection failed: %s", err)
			}
			peer.send <- resp
		case peer := <-s.unregister:
			s.logger.Printf("peer %s disconnected", peer.conn.RemoteAddr())
		case tuple := <-s.message:
			s.logger.Printf("new incomming message %s", string(tuple.msg.Command))
			if err := s.processMessage(tuple.msg, tuple.peer); err != nil {
				s.logger.Fatalf("failed to process message: %s", err)
			}
		case <-s.quit:
			s.shutdown()
		}
	}
}

// TODO: unregister peers on error.
// processMessage processes the received message from a remote node.
func (s *Server) processMessage(msg *Message, peer *Peer) error {
	switch msg.commandType() {
	case cmdVersion:
		v, _ := msg.decodePayload()
		resp, err := s.handleVersionCmd(v.(*Version))
		if err != nil {
			return err
		}
		peer.send <- resp
	case cmdVerack:
	case cmdGetAddr:
	case cmdAddr:
	case cmdGetHeaders:
	case cmdHeaders:
	case cmdGetBlocks:
	case cmdInv:
	case cmdGetData:
	case cmdBlock:
	case cmdTX:
	default:
		return errors.New("invalid RPC command received: " + string(msg.commandType()))
	}

	return nil
}

// When a new peer is connected we respond with the version command.
// No further communication should been made before both sides has received
// the version of eachother.
func (s *Server) handlePeerConnected() (*Message, error) {
	payload := newVersionPayload(s.port(), s.userAgent, 0, s.relay)
	b, err := payload.encode()
	if err != nil {
		return nil, err
	}
	msg := newMessage(ModeTestNet, cmdVersion, b)
	return msg, nil
}

// Version declares the server's version when a new connection is been made.
// We respond with a instant "verack" message.
func (s *Server) handleVersionCmd(v *Version) (*Message, error) {
	// TODO: check version and verify to trust that node.

	// Empty payload for the verack message.
	fmt.Printf("%+v\n", v)

	msg := newMessage(s.netMode, cmdVerack, nil)
	return msg, nil
}

func (s *Server) port() uint16 {
	if s.dev {
		return portDevNet
	}
	if s.netMode == ModeMainNet {
		return portMainNet
	}
	if s.netMode == ModeTestNet {
		return portTestNet
	}

	s.logger.Fatalf("the server dont know what ports it running, yikes.")
	return 0
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
