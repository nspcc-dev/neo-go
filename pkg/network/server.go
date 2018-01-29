package network

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

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
	rpcLogger = log.New(os.Stdout, "", 0)
)

type messageTuple struct {
	peer *Peer
	msg  *Message
}

// Server is the representation of a full working NEO TCP node.
type Server struct {
	logger *log.Logger

	mtx sync.RWMutex

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
		s.unregister <- peer
	}
}

func (s *Server) disconnect(p *Peer) {
	s.unregister <- p
}

func (s *Server) loop() {
	for {
		select {
		case peer := <-s.register:
			// When a new connection is been established, (by this server or remote node)
			// its peer will be received on this channel.
			// Any peer registration must happen via this channel.
			s.logger.Printf("peer registered from address %s", peer.conn.RemoteAddr())

			s.peers[peer] = true

			// Only respond with a version message if the peer initiated the connection.
			if peer.initiator {
				resp, err := s.handlePeerConnected()
				if err != nil {
					s.logger.Fatalf("handling initial peer connection failed: %s", err)
				} else {
					peer.send <- resp
				}
			}
		case peer := <-s.unregister:
			// unregister should take care of all the cleanup that has to be made.
			if _, ok := s.peers[peer]; ok {
				peer.conn.Close()
				close(peer.send)
				delete(s.peers, peer)
				s.logger.Printf("peer %s disconnected", peer.conn.RemoteAddr())
			}
		case tuple := <-s.message:
			// When a remote node sends data over its connection it will be received
			// on this channel.
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
	rpcLogger.Printf("[NODE %d] :: IN :: %s :: %+v", peer.id, msg.commandType(), msg.Payload)

	switch msg.commandType() {
	case cmdVersion:
		return s.handleVersionCmd(msg.Payload.(*payload.Version), peer)
	case cmdVerack:
	case cmdGetAddr:
		// return s.handleGetAddrCmd(msg, peer)
	case cmdAddr:
		return s.handleAddrCmd(msg.Payload.(*payload.AddressList), peer)
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
	peer.id = v.Nonce
	verackMsg := newMessage(s.net, cmdVerack, nil)
	peer.send <- verackMsg

	go s.startProtocol(peer)

	return nil
}

// When the remote node reveals its known peers we try to connect to all of them.
func (s *Server) handleAddrCmd(addrList *payload.AddressList, peer *Peer) error {
	for _, addr := range addrList.Addrs {
		if !s.peerAlreadyConnected(addr.Addr) {
			go connectToRemoteNode(s, addr.Addr.String())
		}
	}
	return nil
}

func (s *Server) peerAlreadyConnected(addr net.Addr) bool {
	// TODO: check for race conditions
	//s.mtx.RLock()
	//defer s.mtx.RUnlock()

	// What about ourself ^^

	for peer := range s.peers {
		if peer.conn.RemoteAddr().String() == addr.String() {
			return true
		}
	}
	return false
}

// After receiving the "getaddr" the server needs to respond with an "addr" message.
// providing information about the other nodes in the network.
// e.g. this server's connected peers.
func (s *Server) handleGetAddrCmd(msg *Message, peer *Peer) error {
	// TODO
	return nil
}

func (s *Server) startProtocol(peer *Peer) {
	// TODO: check if this peer is still connected.
	for {
		getaddrMsg := newMessage(s.net, cmdGetAddr, nil)
		peer.send <- getaddrMsg

		time.Sleep(10 * time.Second)
	}
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
