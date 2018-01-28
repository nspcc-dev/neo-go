package network

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/anthdm/neo-go/pkg/network/payload"
	"github.com/anthdm/neo-go/pkg/util"
)

const (
	// node version
	version = "2.6.0"
	// official ports according to the protocol.
	portMainNet = 10333
	portTestNet = 20333
)

var (
	// rpcLogger used for debugging RPC messages between nodes.
	rpcLogger = log.New(os.Stdout, "RPC :: ", 0)
)

type messageTuple struct {
	peer *Peer
	msg  *Message
}

// Server is the representation of a full working NEO TCP node.
type Server struct {
	logger *log.Logger

	// id of the server
	id uint32

	// the port the TCP listener is listening on.
	port uint16

	// userAgent of the server.
	userAgent string
	// The "magic" mode the server is currently running on.
	// This can either be 0x00746e41 or 0x74746e41 for main or test net.
	// Or 56753 to work with the docker privnet.
	net NetMode
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
}

// NewServer returns a pointer to a new server.
func NewServer(net NetMode) *Server {
	logger := log.New(os.Stdout, "NEO SERVER :: ", 0)

	if net != ModeTestNet && net != ModeMainNet && net != ModeDevNet {
		logger.Fatalf("invalid network mode %d", net)
	}

	s := &Server{
		// It is important to have this user agent correct. Otherwise we will get
		// disconnected.
		id:         util.RandUint32(1111111, 9999999),
		userAgent:  fmt.Sprintf("\v/NEO:%s/", version),
		logger:     logger,
		peers:      make(map[*Peer]bool),
		register:   make(chan *Peer),
		unregister: make(chan *Peer),
		message:    make(chan messageTuple),
		relay:      true,
		net:        net,
		quit:       make(chan struct{}),
	}

	return s
}

// Start run's the server.
func (s *Server) Start(port string, seeds []string) {
	p, err := strconv.Atoi(port[1:len(port)])
	if err != nil {
		s.logger.Fatalf("could not convert port to integer: %s", err)
	}
	s.port = uint16(p)

	fmt.Println(logo())
	fmt.Println(string(s.userAgent))
	fmt.Println("")
	s.logger.Printf("NET: %s - TCP: %d - RELAY: %v - ID: %d",
		s.net, int(s.port), s.relay, s.id)

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

func (s *Server) disconnect(p *Peer) {
	p.conn.Close()
	close(p.send)
	s.unregister <- p
}

func (s *Server) loop() {
	for {
		select {
		case peer := <-s.register:
			s.logger.Printf("peer registered from address %s", peer.conn.RemoteAddr())

			s.peers[peer] = true

			// only respond with the version mesage if the peer initiated the connection.
			if peer.initiator {
				resp, err := s.handlePeerConnected()
				if err != nil {
					s.logger.Fatalf("handling initial peer connection failed: %s", err)
				}
				peer.send <- resp
			}
		case peer := <-s.unregister:
			if _, ok := s.peers[peer]; ok {
				delete(s.peers, peer)
				s.logger.Printf("peer %s disconnected", peer.conn.RemoteAddr())
			}
		case tuple := <-s.message:
			if err := s.processMessage(tuple.msg, tuple.peer); err != nil {
				s.logger.Fatalf("failed to process message: %s", err)
				s.disconnect(tuple.peer)
			}
		case <-s.quit:
			s.shutdown()
		}
	}
}

// TODO: unregister peers on error.
// processMessage processes the received message from a remote node.
func (s *Server) processMessage(msg *Message, peer *Peer) error {
	rpcLogger.Printf("IN :: %s", msg.commandType())
	if msg.Length > 0 {
		rpcLogger.Printf("IN :: %+v", msg.Payload)
	}

	switch msg.commandType() {
	case cmdVersion:
		return s.handleVersionCmd(msg.Payload.(*payload.Version), peer)
	case cmdVerack:
	case cmdGetAddr:
		return s.handleGetAddrCmd(msg, peer)
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
	payload := payload.NewVersion(s.id, s.port, s.userAgent, 0, s.relay)
	msg := newMessage(s.net, cmdVersion, payload)
	return msg, nil
}

// Version declares the server's version.
func (s *Server) handleVersionCmd(v *payload.Version, peer *Peer) error {
	// TODO: check version and verify to trust that node.

	payload := payload.NewVersion(s.id, s.port, s.userAgent, 0, s.relay)
	// we respond with our version.
	versionMsg := newMessage(s.net, cmdVersion, payload)
	peer.send <- versionMsg

	// we respond with a verack, we successfully received peer's version
	// at this point.
	peer.verack = true
	verackMsg := newMessage(s.net, cmdVerack, nil)
	peer.send <- verackMsg

	return nil
}

// After receiving the "getaddr" the server needs to respond with an "addr" message.
// providing information about the other nodes in the network.
// e.g. this server's connected peers.
func (s *Server) handleGetAddrCmd(msg *Message, peer *Peer) error {
	// payload := NewAddrPayload()
	// b, err := payload.encode()
	// if err != nil {
	// 	return err
	// }
	// var addrList []AddrWithTimestamp
	// for peer := range s.peers {
	// 	addrList = append(addrList, newAddrWithTimestampFromPeer(peer))
	// }

	return nil
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
