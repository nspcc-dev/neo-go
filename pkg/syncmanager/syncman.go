// The syncmanager will use a modified verison of the initial block download in bitcoin
// Seen here: https://en.bitcoinwiki.org/wiki/Bitcoin_Core_0.11_(ch_5):_Initial_Block_Download
// MovingWindow is a desired featured from the original codebase

package syncmanager

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/peermanager"

	"github.com/CityOfZion/neo-go/pkg/blockchain"
	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/pubsub"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Moving Variables are OnHeaders and OnBlocks
// And HeadersFirstMode, BlocksOnly and Maintain
/*

HeadersFirstMode + OnHeaders
*/
var (
	// This is the maximum amount of inflight objects that we would like to have
	// Number taken from original codebase
	maxBlockRequest = 1024

	// This is the maximum amount of blocks that we will ask for from a single peer
	// Number taken from original codebase
	maxBlockRequestPerPeer = 16
)

type Syncmanager struct {
	pmgr              *peermanager.PeerMgr
	Mode              int // 1 = headersFirst, 2 = Blocks, 3 = Maintain
	chain             *blockchain.Chain
	headers           []util.Uint256
	inflightBlockReqs map[util.Uint256]*peer.Peer // when we send a req for block, we will put hash in here, along with peer who we requested it from
}

func New(chain *blockchain.Chain, pmgr *peermanager.PeerMgr, bestHash util.Uint256) *Syncmanager {
	return &Syncmanager{
		pmgr,
		1,
		chain,
		[]util.Uint256{},
		make(map[util.Uint256]*peer.Peer, 2000),
	}
}

func (s *Syncmanager) OnHeaders(p *peer.Peer, msg *payload.HeadersMessage) {
	fmt.Println("Sync manager On Headers called")
	// On receipt of Headers
	// check what mode we are in
	// HeadersMode, we check if there is 2k. If so call again. If not then change mode into BlocksOnly
	if s.Mode == 1 {
		err := s.HeadersFirstMode(p, msg)
		if err != nil {
			fmt.Println("Error re blocks", err)
			return // We should custom name error so, that we can do something on WrongHash Error, Peer disconnect error
		}
		return

	}

	// If we receive a Header while in BlocksOnly mode
	// We just save the headers.
	if s.Mode == 2 {
		// Just save header while in BlocksOnly Mode
		// err := s.addHeaders(msg)
		// if err != nil {
		// 	p.Disconnect()
		// 	fmt.Println("Bad Header, disconnecting Peer")
		// }

	}

	// Maintain: We may receive headers when in maintain mode
	// We save it and ask for the block that goes with it
	if s.Mode == 3 {

	}

}

func (s *Syncmanager) HeadersFirstMode(p *peer.Peer, msg *payload.HeadersMessage) error {

	fmt.Println("Headers first mode")

	// Validate Headers
	err := s.chain.ValidateHeaders(msg)

	if err != nil {
		// Re-request headers from a different peer
		s.pmgr.Disconnect(p)
		fmt.Println("Error Validating headers", err)
		return err
	}

	// Add Headers into db
	err = s.chain.AddHeaders(msg)
	if err != nil {
		// Try addding them into the db again?
		// Since this is simply a db insert, any problems here means trouble
		//TODO(KEV) : Should we Switch off system or warn the user that the system is corrupted?
		fmt.Println("Error Adding headers", err)

		//TODO: Batching is not yet implemented,
		// So here we would need to remove headers which have been added
		// from the slice
		return err
	}

	// Add header hashes into slice
	// Requets first batch of blocks here
	var hashes []util.Uint256
	for _, header := range msg.Headers {
		hashes = append(hashes, header.Hash)
	}
	s.headers = append(s.headers, hashes...)

	if len(msg.Headers) == 2*1e3 { // should be less than 2000, leave it as this for tests
		fmt.Println("Switching to BlocksOnly Mode")
		s.Mode = 2 // switch to BlocksOnly. XXX: because HeadersFirst is not in parallel, no race condition here.
		return s.RequestMoreBlocks()
	}
	lastHeader := msg.Headers[len(msg.Headers)-1]
	return s.pmgr.RequestHeaders(lastHeader.Hash)
}

func (s *Syncmanager) RequestMoreBlocks() error {

	var blockReq []util.Uint256

	var reqAmount int

	if len(s.headers) >= maxBlockRequestPerPeer {
		reqAmount = maxBlockRequestPerPeer
		blockReq = s.headers[:reqAmount]
	} else {
		reqAmount = len(s.headers)
		blockReq = s.headers[:reqAmount]
	}
	peer, err := s.pmgr.RequestBlocks(blockReq)
	if err != nil { // This could happen if the peermanager has no valid peers to connect to. We should wait a bit and re-request
		return err // alternatively we could make RequestBlocks blocking, then make sure it is not triggered when a block is received
	}

	//XXX: Possible race condition, between us requesting the block and adding it to
	// the inflight block map? Give that node a medal.

	for _, hash := range s.headers {
		s.inflightBlockReqs[hash] = peer
	}
	s.headers = s.headers[reqAmount:]
	// NONONO: Here we do not pass all of the hashes to peermanager because
	// it is not the peermanagers responsibility to mange inflight blocks
	return err
}

// OnBlock receives a block from a peer, then passes it to the blockchain to process.
// For now, this will be blocking. When made async, ensure that if a bad block
// Is received from a peer, we disconnect the peer, remove all blocks in flight from peer
// and re-request those blocks. Any blocks that are now relying on those re-requested blocks will now need to wait
func (s *Syncmanager) OnBlock(p *peer.Peer, msg *payload.BlockMessage) {
	// We should not deal with chain state in here
	// If a block fails, we will be notified by the pubsub system, to request it again.

	if s.Mode == 1 {
		// Peers will broadcast blocks
		// when they find a new block, we will ignore them while in this mode
	} else if s.Mode == 2 || s.Mode == 3 {
		err := s.chain.AddBlock(msg)
		if err != nil {
			// Put headers back in front of queue to fetch block for.
			fmt.Println("Block had an error", err)
		}

		if len(s.inflightBlockReqs) < maxBlockRequest && len(s.inflightBlockReqs) != 0 {
			err := s.RequestMoreBlocks()
			if err != nil {
				fmt.Println("Could not request more blocks", err)
			}
		}

		if len(s.headers) < 120 {
			// original NEO code base uses 2k to signal maintain mode.
			// I would like to stay in parallel download mode for as long as possible
			// To speed up syncing
			// TODO(KEV): instead of using the number of headers, use the blocktimestamp
			// If timestamp of a block is within the hour or two then we switch.
			// Although we download in parallel, The difference between the lowest and highest block
			// should be consistent.
			// For this to happen, the MovingWindow will need to be implemented.
			s.Mode = 3
		}
	}
}

// Topics will implement the subscriber interface
// for pub-sub. Return type is a list of Topics that
// The syncmanager is interested in receiving information
//about. Can be static or dynamic by making it inside of the syncmanager struct
func (s *Syncmanager) Topics() []pubsub.EventType {
	return []pubsub.EventType{}
}

// Emit will Receive events from Whoever they have subscribed to
func (s *Syncmanager) Emit(e pubsub.Event) {
	if e.Type == pubsub.NewBlock {
		// do something we have a new block
	}
	// do something with received event
}

/*
	Stage 1: We download the headers only from one peer. We make sure headers fit and save them, then take the hash
	of header and put it in the wantedBlocks list.

	<< This means we can put the addHeaders method back into Chain and save headers to disk.>>

	Stage 2: We are now in BlocksOnly Mode, We start requesting blocks in bulks of 21 from peers
	from the wantedHeaders list. When we receive a block, we save it and if it is the next block that the blockchain needs
	We give it to the chain. If not we save it. Or we could use channels, make channel a size of 2k and send them over channel
	Letting the blockchain deal with newBlocks via the channel, send async over channel
	if blockchain says all is good, then we continue. Problem is that if we have queued up a lot of blocks and they are invalid

	We need to keep track of who we have requested blocks from, incase they disconnect, then re-request from
	different peers.

	map: key = peer and val = slice of blocks Requested

	If a peer is disconnected, we take
*/

/*
1. Download the headers from one peer inititally
2. When downloaded verify and save them
3. Switch to BlocksOnly Mode
4. Fetch multiple blocks in parallel

We will have a PeerManager, which will inform others of when a Peer disconnects. Whoever wants to know
When the SyncManager sees that a peer has disconnected, then we put their blocks back to the front
of the queue.

The sync manager will not be care about the peers it will ask the peer manager to manage the peer states.

The syncmanager will be in charge of synchronising the node, when blocks are recieved they are passed onto
the blockchain to process. The syncmanager should receive the blocks and headers because it also keeps the state
of requested blocks.

5. Maintain mode:

- If we are in this mode and we receive headers, we disconnect the peer, as we did not ask for
them.
- If we receive blocks, then pass them onto the chain


HEADERS Mode:

OnHeaders: Save the header and make sure it links with previous
OnBlock : disconnect Peer

BLOCKSONLY:

OnHeaders: Disconnect, we did not ask for headers
OnBlock: Match it with a header, if it is a block that is > headerHeight just discard,
If it is equal <= headerHeight but just invalid block, then disconnect peer.
If matches a header, save and validate

MAINTAIN:

OnHeaders: Save header and validate, then send a getdata for the block that corresponds to it
OnBlock: Save and validate

https://bitcointalk.org/index.php?topic=550.0 - Batch save instead of saving 1 at a time. Can we
save the intermediate state of blocks , based on previous blocks, then batch save the state

*/
