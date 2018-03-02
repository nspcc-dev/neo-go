package network

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
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

// TODO: Maybe util is a better place for such data types.
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

	if net != ModeTestNet && net != ModeMainNet && net != ModePrivNet {
		logger.Fatalf("invalid network mode %d", net)
	}

	// For now I will hard code a genesis block of the docker privnet container.
	startHash, _ := util.Uint256DecodeString("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")

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
		bc:          core.NewBlockchain(core.NewMemoryStore(), logger, startHash),
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
		peer.disconnect()
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

				if err := s.handlePeerConnected(peer); err != nil {
					s.logger.Printf("failed handling peer connection: %s", err)
					peer.disconnect()
				}
			}

		// unregister safely deletes a peer. For disconnecting peers use the
		// disconnect() method on the peer, it will call unregister and terminates its routines.
		case peer := <-s.unregister:
			if _, ok := s.peers[peer]; ok {
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
func (s *Server) handlePeerConnected(p Peer) error {
	// TODO: get the blockheight of this server once core implemented this.
	payload := payload.NewVersion(s.id, s.port, s.userAgent, s.bc.HeaderHeight(), s.relay)
	msg := newMessage(s.net, cmdVersion, payload)
	return p.callVersion(msg)
}

func (s *Server) handleVersionCmd(version *payload.Version, p Peer) error {
	if s.id == version.Nonce {
		return errors.New("identical nonce")
	}
	if p.addr().Port != version.Port {
		return fmt.Errorf("port mismatch: %d and %d", version.Port, p.addr().Port)
	}

	return p.callVerack(newMessage(s.net, cmdVerack, nil))
}

func (s *Server) handleGetaddrCmd(msg *Message, p Peer) error {
	return nil
}

// The node can broadcast the object information it owns by this message.
// The message can be sent automatically or can be used to answer getbloks messages.
func (s *Server) handleInvCmd(inv *payload.Inventory, p Peer) error {
	if !inv.Type.Valid() {
		return fmt.Errorf("invalid inventory type %s", inv.Type)
	}
	if len(inv.Hashes) == 0 {
		return errors.New("inventory should have at least 1 hash got 0")
	}

	// todo: only grab the hashes that we dont know.

	payload := payload.NewInventory(inv.Type, inv.Hashes)
	resp := newMessage(s.net, cmdGetData, payload)

	return p.callGetdata(resp)
}

// handleBlockCmd processes the received block.
func (s *Server) handleBlockCmd(block *core.Block, p Peer) error {
	hash, err := block.Hash()
	if err != nil {
		return err
	}

	s.logger.Printf("new block: index %d hash %s", block.Index, hash)

	return nil
}

// After receiving the getaddr message, the node returns an addr message as response
// and provides information about the known nodes on the network.
func (s *Server) handleAddrCmd(addrList *payload.AddressList, p Peer) error {
	for _, addr := range addrList.Addrs {
		if !s.peerAlreadyConnected(addr.Addr) {
			// TODO: this is not transport abstracted.
			go connectToRemoteNode(s, addr.Addr.String())
		}
	}
	return nil
}

// Handle the headers received from the remote after we asked for headers with the
// "getheaders" message.
func (s *Server) handleHeadersCmd(headers *payload.Headers, p Peer) error {
	// Set a deadline for adding headers?
	go func(ctx context.Context, headers []*core.Header) {
		if err := s.bc.AddHeaders(headers...); err != nil {
			s.logger.Printf("failed to add headers: %s", err)
			return
		}

		// Ask more headers if we are not in sync with the peer.
		if s.bc.HeaderHeight() < p.version().StartHeight {
			if err := s.askMoreHeaders(p); err != nil {
				s.logger.Printf("getheaders RPC failed: %s", err)
				return
			}
		}
	}(context.TODO(), headers.Hdrs)

	return nil
}

// Ask the peer for more headers We use the current block hash as start.
func (s *Server) askMoreHeaders(p Peer) error {
	start := []util.Uint256{s.bc.CurrentHeaderHash()}
	payload := payload.NewGetBlocks(start, util.Uint256{})
	msg := newMessage(s.net, cmdGetHeaders, payload)

	return p.callGetheaders(msg)
}

// check if the addr is already connected to the server.
func (s *Server) peerAlreadyConnected(addr net.Addr) bool {
	// TODO: Dont try to connect with ourselfs.
	for peer := range s.peers {
		if peer.addr().String() == addr.String() {
			return true
		}
	}
	return false
}

// TODO: Quit this routine if the peer is disconnected.
func (s *Server) startProtocol(p Peer) {
	if s.bc.HeaderHeight() < p.version().StartHeight {
		s.askMoreHeaders(p)
	}
	for {
		getaddrMsg := newMessage(s.net, cmdGetAddr, nil)
		p.callGetaddr(getaddrMsg)

		time.Sleep(30 * time.Second)
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
