package peermgr

import (
	"errors"
	"fmt"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/wire/command"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

const (
	// blockCacheLimit is the maximum amount of pending requests that the cache can hold
	pendingBlockCacheLimit = 20

	//peerBlockCacheLimit is the maximum amount of inflight blocks that a peer can
	// have, before they are flagged as busy
	peerBlockCacheLimit = 1
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
	// when a peer is sent a blockRequest
	// the peermanager will track this using this blockCache
	blockCache *blockCache
	// all other requests will be tracked using the requests map
	requests map[command.Type]bool
}

//PeerMgr manages all peers that the node is connected to
type PeerMgr struct {
	pLock sync.RWMutex
	peers map[mPeer]peerstats

	requestCache *blockCache
}

//New returns a new peermgr object
func New() *PeerMgr {
	return &PeerMgr{
		peers:        make(map[mPeer]peerstats),
		requestCache: newBlockCache(pendingBlockCacheLimit),
	}
}

// AddPeer adds a peer to the list of managed peers
func (pmgr *PeerMgr) AddPeer(peer mPeer) {

	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()
	if _, exists := pmgr.peers[peer]; exists {
		return
	}
	pmgr.peers[peer] = peerstats{
		requests:   make(map[command.Type]bool),
		blockCache: newBlockCache(peerBlockCacheLimit),
	}
	go pmgr.onDisconnect(peer)
}

//MsgReceived notifies the peer manager that we have received a
// message from a peer
func (pmgr *PeerMgr) MsgReceived(peer mPeer, cmd command.Type) error {
	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()

	// if peer was unknown then disconnect
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

//BlockMsgReceived notifies the peer manager that we have received a
// block message from a peer
func (pmgr *PeerMgr) BlockMsgReceived(peer mPeer, bi BlockInfo) error {

	// if peer was unknown then disconnect
	val, ok := pmgr.peers[peer]
	if !ok {

		go func() {
			peer.NotifyDisconnect()
		}()

		peer.Disconnect()
		return ErrUnknownPeer
	}

	// // remove item from the peersBlock cache
	err := val.blockCache.removeHash(bi.BlockHash)
	if err != nil {
		return err
	}

	// check if cache empty, if so then return
	if pmgr.requestCache.cacheLen() == 0 {
		return nil
	}

	// Try to clean an item from the pendingBlockCache, a peer has just finished serving a block request
	cachedBInfo, err := pmgr.requestCache.pickFirstItem()
	if err != nil {
		return err
	}

	return pmgr.blockCallPeer(cachedBInfo, func(p mPeer) error {
		return p.RequestBlocks([]util.Uint256{cachedBInfo.BlockHash})
	})
}

// Len returns the amount of peers that the peer manager
//currently knows about
func (pmgr *PeerMgr) Len() int {
	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()
	return len(pmgr.peers)
}

// RequestBlock will request a block from the most
// available peer. Then update it's stats, so we know that
// this peer is busy
func (pmgr *PeerMgr) RequestBlock(bi BlockInfo) error {
	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()

	err := pmgr.blockCallPeer(bi, func(p mPeer) error {
		return p.RequestBlocks([]util.Uint256{bi.BlockHash})
	})

	if err == ErrNoAvailablePeers {
		return pmgr.requestCache.addBlockInfo(bi)
	}

	return err
}

// RequestHeaders will request a headers from the most available peer.
func (pmgr *PeerMgr) RequestHeaders(hash util.Uint256) error {
	pmgr.pLock.Lock()
	defer pmgr.pLock.Unlock()
	return pmgr.callPeerForCmd(command.Headers, func(p mPeer) error {
		return p.RequestHeaders(hash)
	})
}

func (pmgr *PeerMgr) callPeerForCmd(cmd command.Type, f func(p mPeer) error) error {
	for peer, stats := range pmgr.peers {
		if !stats.requests[cmd] {
			stats.requests[cmd] = true
			return f(peer)
		}
	}
	return ErrNoAvailablePeers
}

func (pmgr *PeerMgr) blockCallPeer(bi BlockInfo, f func(p mPeer) error) error {
	for peer, stats := range pmgr.peers {
		if stats.blockCache.cacheLen() < peerBlockCacheLimit {
			err := stats.blockCache.addBlockInfo(bi)
			if err != nil {
				return err
			}
			return f(peer)
		}
	}
	return ErrNoAvailablePeers
}

func (pmgr *PeerMgr) onDisconnect(p mPeer) {

	// Blocking until peer is disconnected
	p.NotifyDisconnect()

	pmgr.pLock.Lock()
	defer func() {
		delete(pmgr.peers, p)
		pmgr.pLock.Unlock()
	}()

	// Add all of peers outstanding block requests into
	// the peer managers pendingBlockRequestCache

	val, ok := pmgr.peers[p]
	if !ok {
		return
	}

	pendingRequests, err := val.blockCache.pickAllItems()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = pmgr.requestCache.addBlockInfos(pendingRequests)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
}
