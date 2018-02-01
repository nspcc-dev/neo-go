package network

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/anthdm/neo-go/pkg/core"
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
	// channel for safely responding the number of current connected peers.
	peerCountCh chan peerCount
	// a list of hashes that
	knownHashes protectedHashmap
	// The blockchain.
	bc *core.Blockchain
}

type protectedHashmap struct {
	*sync.RWMutex
	hashes map[util.Uint256]bool
}

func (m protectedHashmap) add(h util.Uint256) bool {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.hashes[h]; !ok {
		m.hashes[h] = true
		return true
	}
	return false
}

func (m protectedHashmap) remove(h util.Uint256) bool {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.hashes[h]; ok {
		delete(m.hashes, h)
		return true
	}
	return false
}

func (m protectedHashmap) has(h util.Uint256) bool {
	m.RLock()
	defer m.RUnlock()

	_, ok := m.hashes[h]

	return ok
}

// NewServer returns a pointer to a new server.
func NewServer(net NetMode) *Server {
	logger := log.New(os.Stdout, "[NEO SERVER] :: ", 0)

	if net != ModeTestNet && net != ModeMainNet && net != ModeDevNet {
		logger.Fatalf("invalid network mode %d", net)
	}

	s := &Server{
		id:          util.RandUint32(1111111, 9999999),
		userAgent:   fmt.Sprintf("/NEO:%s/", version),
		logger:      logger,
		peers:       make(map[Peer]bool),
		register:    make(chan Peer),
		unregister:  make(chan Peer),
		message:     make(chan messageTuple),
		relay:       true, // currently relay is not handled.
		net:         net,
		quit:        make(chan struct{}),
		peerCountCh: make(chan peerCount),
		// knownHashes: protectedHashmap{},
		bc: core.NewBlockchain(core.NewMemoryStore()),
	}

	return s
}

// Start run's the server.
// TODO: server should be initialized with a config.
func (s *Server) Start(opts StartOpts) {
	s.port = uint16(opts.TCP)

	fmt.Println(logo())
	fmt.Println(string(s.userAgent))
	fmt.Println("")
	s.logger.Printf("NET: %s - TCP: %d - RELAY: %v - ID: %d",
		s.net, int(s.port), s.relay, s.id)

	go listenTCP(s, opts.TCP)

	if opts.RPC > 0 {
		go listenHTTP(s, opts.RPC)
	}

	if len(opts.Seeds) > 0 {
		connectToSeeds(s, opts.Seeds)
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

		case t := <-s.peerCountCh:
			t.count <- len(s.peers)

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

func (s *Server) handleVersionCmd(msg *Message, p Peer) *Message {
	version := msg.Payload.(*payload.Version)
	if s.id == version.Nonce {
		// s.unregister <- p
		return nil
	}
	if p.addr().Port != version.Port {
		// s.unregister <- p
		return nil
	}
	return newMessage(ModeDevNet, cmdVerack, nil)
}

func (s *Server) handleGetaddrCmd(msg *Message, p Peer) *Message {
	return nil
}

// The node can broadcast the object information it owns by this message.
// The message can be sent automatically or can be used to answer getbloks messages.
func (s *Server) handleInvCmd(msg *Message, p Peer) *Message {
	inv := msg.Payload.(*payload.Inventory)
	if !inv.Type.Valid() {
		s.unregister <- p
		return nil
	}
	if len(inv.Hashes) == 0 {
		s.unregister <- p
		return nil
	}

	// todo: only grab the hashes that we dont know.

	payload := payload.NewInventory(inv.Type, inv.Hashes)
	resp := newMessage(s.net, cmdGetData, payload)
	return resp
}

// handleBlockCmd processes the received block.
func (s *Server) handleBlockCmd(msg *Message, p Peer) {
	block := msg.Payload.(*core.Block)
	hash, err := block.Hash()
	if err != nil {
		// not quite sure what to do here.
		// should we disconnect the client or just silently log and move on?
		s.logger.Printf("failed to generate block hash: %s", err)
		return
	}

	fmt.Println(hash)

	if s.bc.HasBlock(hash) {
		return
	}
}

// After receiving the getaddr message, the node returns an addr message as response
// and provides information about the known nodes on the network.
func (s *Server) handleAddrCmd(msg *Message, p Peer) {
	addrList := msg.Payload.(*payload.AddressList)
	for _, addr := range addrList.Addrs {
		if !s.peerAlreadyConnected(addr.Addr) {
			// TODO: this is not transport abstracted.
			go connectToRemoteNode(s, addr.Addr.String())
		}
	}
}

func (s *Server) relayInventory(inv *payload.Inventory) {
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
	// dont keep asking (maxPeers and no new nodes)
	for {
		getaddrMsg := newMessage(s.net, cmdGetAddr, nil)
		peer.callGetaddr(getaddrMsg)

		time.Sleep(120 * time.Second)
	}
}

type peerCount struct {
	count chan int
}

// peerCount returns the number of connected peers to this server.
func (s *Server) peerCount() int {
	ch := peerCount{
		count: make(chan int),
	}

	s.peerCountCh <- ch

	return <-ch.count
}

// StartOpts holds the server configuration.
type StartOpts struct {
	// tcp port
	TCP int
	// slice of peer addresses the server will connect to
	Seeds []string
	// JSON-RPC port. If 0 no RPC handler will be attached.
	RPC int
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
