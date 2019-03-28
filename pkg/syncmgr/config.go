package syncmgr

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Config is the configuration file for the sync manager
type Config struct {

	// Chain functions
	ProcessBlock   func(msg payload.Block) error
	ProcessHeaders func(hdrs []*payload.BlockBase) error

	// RequestHeaders will send a getHeaders request
	// with the hash passed in as a parameter
	RequestHeaders func(hash util.Uint256) error

	//RequestBlock will send a getdata request for the block
	// with the hash passed as a parameter
	RequestBlock func(hash util.Uint256) error

	// GetNextBlockHash returns the block hash of the header infront of thr block
	// at the tip of this nodes chain. This assumes that the node is not in sync
	GetNextBlockHash func() (util.Uint256, error)

	// GetBestBlockHash gets the block hash of the last saved block.
	GetBestBlockHash func() (util.Uint256, error)

	// AskForNewBlocks will send out a message to the network
	// asking for new blocks
	AskForNewBlocks func()

	// FetchHeadersAgain is called when a peer has provided headers that have not
	// validated properly. We pass in the hash of the first header
	FetchHeadersAgain func(util.Uint256) error

	// FetchHeadersAgain is called when a peer has provided a block that has not
	// validated properly. We pass in the hash of the block
	FetchBlockAgain func(util.Uint256) error
}

// SyncPeer represents a peer on the network
// that this node can sync with
type SyncPeer interface {
	RequestBlocks(hashes []util.Uint256) error
	RequestHeaders(hash util.Uint256) error
	Height() uint32
}
