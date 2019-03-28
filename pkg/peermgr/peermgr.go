package peermgr

import (
	"errors"
	"fmt"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/wire/command"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

var (
	//ErrNoAvailablePeers is returned when a request for data from a peer is invoked
	// but there are no available peers to request data from
	ErrNoAvailablePeers = errors.New("there are no available peers to interact with")

	// ErrUnknownPeer is returned when a peer that the peer manager does not know about
	// sends a message to this node
	ErrUnknownPeer = errors.New("this peer has not been registered with the peer manager")
)

//mPeer represents a peer that is managed by the peer manager
type mPeer interface {
	Disconnect()
	RequestBlocks([]util.Uint256) error
	RequestHeaders(util.Uint256) error
	NotifyDisconnect()
}

type peerstats struct {
	requests map[command.Type]bool
}

//PeerMgr manages all peers that the node is connected to
type PeerMgr struct {
	pLock sync.RWMutex
	peers map[mPeer]peerstats
}

//New returns a new peermgr object
func New() *PeerMgr {
	return &PeerMgr{
		peers: make(map[mPeer]peerstats),
	}
}

// AddPeer adds a peer to the list of managed peers
func (pmgr *PeerMgr) AddPeer(peer mPeer) {

	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()
	if _, exists := pmgr.peers[peer]; exists {
		return
	}
	pmgr.peers[peer] = peerstats{requests: make(map[command.Type]bool)}
	go pmgr.onDisconnect(peer)
}

//MsgReceived notifies the peer manager that we have received a
// message from a peer
func (pmgr *PeerMgr) MsgReceived(peer mPeer, cmd command.Type) error {
	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()
	val, ok := pmgr.peers[peer]
	if !ok {

		go func() {
			peer.NotifyDisconnect()
		}()

		peer.Disconnect()
		return ErrUnknownPeer
	}
	val.requests[cmd] = false

	return nil
}

// Len returns the amount of peers that the peer manager
//currently knows about
func (pmgr *PeerMgr) Len() int {
	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()
	return len(pmgr.peers)
}

// RequestBlock will request  a block from the most
// available peer. Then update it's stats, so we know that
// this peer is busy
func (pmgr *PeerMgr) RequestBlock(hash util.Uint256) error {
	return pmgr.callPeerForCmd(command.Block, func(p mPeer) error {
		return p.RequestBlocks([]util.Uint256{hash})
	})
}

// RequestHeaders will request a headers from the most available peer.
func (pmgr *PeerMgr) RequestHeaders(hash util.Uint256) error {
	return pmgr.callPeerForCmd(command.Headers, func(p mPeer) error {
		return p.RequestHeaders(hash)
	})
}

func (pmgr *PeerMgr) callPeerForCmd(cmd command.Type, f func(p mPeer) error) error {
	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()
	for peer, stats := range pmgr.peers {
		if !stats.requests[cmd] {
			stats.requests[cmd] = true
			return f(peer)
		}
	}
	return ErrNoAvailablePeers
}
func (pmgr *PeerMgr) onDisconnect(p mPeer) {

	// Blocking until peer is disconnected
	p.NotifyDisconnect()

	pmgr.pLock.Lock()
	delete(pmgr.peers, p)
	pmgr.pLock.Unlock()
	fmt.Println(pmgr.Len())
}
