package syncmgr

import (
	"fmt"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type mode uint8

// Note: this is the unoptimised version without parallel sync
// The algorithm for the unoptimsied version is simple:
// Download 2000 headers, then download the blocks for those headers
// Once those blocks are downloaded, we repeat the process again
// Until we are nomore than one block behind the tip.
// Once this happens, we switch into normal mode.
//In normal mode, we have a timer on for X seconds and ask nodes for blocks and also to doublecheck
// if we are behind once the timer runs out.
// The timer restarts whenever we receive a block.
// The parameter X should be approximately the time it takes the network to reach consensus

//blockTimer approximates to how long it takes to reach consensus and propagate
// a block in the network. Once a node has synchronised with the network, he will
// ask the network for a newblock every blockTimer

const blockTimer = 20 * time.Second

// trailingHeight indicates how many blocks the node has to be behind by
// before he switches to headersMode.
const trailingHeight = 100

// indicates how many blocks the node has to be behind by
// before he switches to normalMode and fetches blocks every X seconds.
const cruiseHeight = 0

const (
	headersMode mode = 1
	blockMode   mode = 2
	normalMode  mode = 3
)

//Syncmgr keeps the node in sync with the rest of the network
type Syncmgr struct {
	syncmode mode
	cfg      *Config
	timer    *time.Timer

	// headerHash is the hash of the last header in the last OnHeaders message that we received.
	// When receiving blocks, we can use this to determine whether the node has downloaded
	// all of the blocks for the last headers messages
	headerHash util.Uint256
}

// New creates a new sync manager
func New(cfg *Config) *Syncmgr {

	newBlockTimer := time.AfterFunc(blockTimer, func() {
		cfg.AskForNewBlocks()
	})
	newBlockTimer.Stop()

	return &Syncmgr{
		syncmode: headersMode,
		cfg:      cfg,
		timer:    newBlockTimer,
	}
}

// OnHeader is called when the node receives a headers message
func (s *Syncmgr) OnHeader(peer SyncPeer, msg *payload.HeadersMessage) {

	// XXX(Optimisation): First check if we actually need these headers
	// Check the last header in msg and then check what our latest header that was saved is
	// If our latest header is above the lastHeader, then we do not save it
	// We could also have that our latest header is above only some of the headers.
	// In this case, we should remove the headers that we already have

	if len(msg.Headers) == 0 {
		// XXX: Increment banScore for this peer, for sending empty headers message
		return
	}

	var err error

	switch s.syncmode {
	case headersMode:
		err = s.headersModeOnHeaders(peer, msg.Headers)
	case blockMode:
		err = s.blockModeOnHeaders(peer, msg.Headers)
	case normalMode:
		err = s.normalModeOnHeaders(peer, msg.Headers)
	default:
		err = s.headersModeOnHeaders(peer, msg.Headers)
	}

	// XXX(Kev):The only meaningful error here would be if the peer
	// we re-requested blocks from failed. In the next iteration, this will be handled
	// by the peer manager, who will only return an error, if we are connected to no peers.
	// Upon re-alising this, the node will then send out GetAddresses to the network and
	// syncing will be resumed, once we find peers to connect to.

	if err != nil {
		// just log the error
		fmt.Println(err.Error())
	}

	hdr := msg.Headers[len(msg.Headers)-1]
	fmt.Printf("Finished processing headers. LastHash in set was: %s\n ", hdr.Hash.ReverseString())
}

// OnBlock is called when the node receives a block
func (s *Syncmgr) OnBlock(peer SyncPeer, msg *payload.BlockMessage) {
	fmt.Printf("Block received with height %d\n", msg.Block.Index)

	var err error

	switch s.syncmode {
	case headersMode:
		err = s.headersModeOnBlock(peer, msg.Block)
	case blockMode:
		err = s.blockModeOnBlock(peer, msg.Block)
	case normalMode:
		err = s.normalModeOnBlock(peer, msg.Block)
	default:
		err = s.headersModeOnBlock(peer, msg.Block)
	}

	if err != nil {
		fmt.Println(err.Error())
	}

	fmt.Printf("Processed Block with height %d\n", msg.Block.Index)
}

//IsCurrent returns true if the node is currently
// synced up with the network
func (s *Syncmgr) IsCurrent() bool {
	return s.syncmode == normalMode
}
