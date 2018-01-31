package network

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
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
	maxPeers    = 50
)

type messageTuple struct {
	peer Peer
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
	peers map[Peer]bool
	// channel for handling new registerd peers.
	register chan Peer
	// channel for safely removing and disconnecting peers.
	unregister chan Peer
	// channel for coordinating messages.
	message chan messageTuple
	// channel used to gracefull shutdown the server.
	quit chan struct{}
	// Whether this server will receive and forward messages.
	relay bool
	// TCP listener of the server
	listener net.Listener

	// RPC channels
	versionCh chan versionTuple
	getaddrCh chan getaddrTuple
	invCh     chan invTuple
	addrCh    chan addrTuple
}

// NewServer returns a pointer to a new server.
func NewServer(net NetMode) *Server {
	logger := log.New(os.Stdout, "[NEO SERVER] :: ", 0)

	if net != ModeTestNet && net != ModeMainNet && net != ModeDevNet {
		logger.Fatalf("invalid network mode %d", net)
	}

	s := &Server{
		id:         util.RandUint32(1111111, 9999999),
		userAgent:  fmt.Sprintf("/NEO:%s/", version),
		logger:     logger,
		peers:      make(map[Peer]bool),
		register:   make(chan Peer),
		unregister: make(chan Peer),
		message:    make(chan messageTuple),
		relay:      true, // currently relay is not handled.
		net:        net,
		quit:       make(chan struct{}),
		versionCh:  make(chan versionTuple),
		getaddrCh:  make(chan getaddrTuple),
		invCh:      make(chan invTuple),
		addrCh:     make(chan addrTuple),
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

func (s *Server) loop() {
	for {
		select {
		// When a new connection is been established, (by this server or remote node)
		// its peer will be received on this channel.
		// Any peer registration must happen via this channel.
		case peer := <-s.register:
			if len(s.peers) < maxPeers {
				s.logger.Printf("peer registered from address %s", peer.addr())
				s.peers[peer] = true
				s.handlePeerConnected(peer)
			}

		// Unregister should take care of all the cleanup that has to be made.
		case peer := <-s.unregister:
			if _, ok := s.peers[peer]; ok {
				peer.disconnect()
				delete(s.peers, peer)
				s.logger.Printf("peer %s disconnected", peer.addr())
			}

		// Process the received version and respond with a verack.
		case t := <-s.versionCh:
			if s.id == t.request.Nonce {
				t.peer.disconnect()
			}
			if t.peer.addr().Port != t.request.Port {
				t.peer.disconnect()
			}
			t.response <- newMessage(ModeDevNet, cmdVerack, nil)

		// Process the getaddr cmd.
		case t := <-s.getaddrCh:
			t.response <- &Message{} // just for now.

		// Process the addr cmd. Register peer will handle the maxPeers connected.
		case t := <-s.addrCh:
			for _, addr := range t.request.Addrs {
				if !s.peerAlreadyConnected(addr.Addr) {
					// TODO: this is not transport abstracted.
					go connectToRemoteNode(s, addr.Addr.String())
				}
			}
			t.response <- true

		// Process inventories cmd.
		case t := <-s.invCh:
			if !t.request.Type.Valid() {
				t.peer.disconnect()
				break
			}
			if len(t.request.Hashes) == 0 {
				t.peer.disconnect()
				break
			}

			payload := payload.NewInventory(t.request.Type, t.request.Hashes)
			msg := newMessage(s.net, cmdGetData, payload)
			t.response <- msg

		case <-s.quit:
			s.shutdown()
		}
	}
}

// When a new peer is connected we send our version.
// No further communication should be made before both sides has received
// the versions of eachother.
func (s *Server) handlePeerConnected(p Peer) {
	// TODO: get the blockheight of this server once core implemented this.
	payload := payload.NewVersion(s.id, s.port, s.userAgent, 0, s.relay)
	msg := newMessage(s.net, cmdVersion, payload)
	p.callVersion(msg)
}

type versionTuple struct {
	peer     Peer
	request  *payload.Version
	response chan *Message
}

func (s *Server) handleVersionCmd(msg *Message, p Peer) *Message {
	t := versionTuple{
		peer:     p,
		request:  msg.Payload.(*payload.Version),
		response: make(chan *Message),
	}

	s.versionCh <- t

	return <-t.response
}

type getaddrTuple struct {
	peer     Peer
	request  *Message
	response chan *Message
}

func (s *Server) handleGetaddrCmd(msg *Message, p Peer) *Message {
	t := getaddrTuple{
		peer:     p,
		request:  msg,
		response: make(chan *Message),
	}

	s.getaddrCh <- t

	return <-t.response
}

type invTuple struct {
	peer     Peer
	request  *payload.Inventory
	response chan *Message
}

func (s *Server) handleInvCmd(msg *Message, p Peer) *Message {
	t := invTuple{
		request:  msg.Payload.(*payload.Inventory),
		response: make(chan *Message),
	}

	s.invCh <- t

	return <-t.response
}

type addrTuple struct {
	request  *payload.AddressList
	response chan bool
}

func (s *Server) handleAddrCmd(msg *Message, p Peer) bool {
	t := addrTuple{
		request:  msg.Payload.(*payload.AddressList),
		response: make(chan bool),
	}

	s.addrCh <- t

	return <-t.response
}

// check if the addr is already connected to the server.
func (s *Server) peerAlreadyConnected(addr net.Addr) bool {
	for peer := range s.peers {
		if peer.addr().String() == addr.String() {
			return true
		}
	}
	return false
}

func (s *Server) sendLoop(peer Peer) {
	// TODO: check if this peer is still connected.
	for {
		getaddrMsg := newMessage(s.net, cmdGetAddr, nil)
		peer.callGetaddr(getaddrMsg)

		time.Sleep(120 * time.Second)
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
