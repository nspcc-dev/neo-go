package syncmgr

import (
	"github.com/CityOfZion/neo-go/pkg/chain"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

// blockModeOnBlock is called when the sync manager is block mode
// and receives a block.
func (s *Syncmgr) blockModeOnBlock(peer SyncPeer, block payload.Block) error {

	// Process Block
	err := s.cfg.ProcessBlock(block)

	if err == chain.ErrFutureBlock {
		// XXX(Optimisation): We can cache future blocks in blockmode, if we have the corresponding header
		// We can have the server cache them and sort out the semantics for when to send them to the chain
		// Server can listen on chain for when a new block is saved
		// or we could embed a struct in this syncmgr called blockCache, syncmgr can just tell it when it has processed
		//a block and we can call ProcessBlock
		return err
	}

	if err != nil && err != chain.ErrBlockAlreadyExists {
		return s.cfg.FetchBlockAgain(block.Hash)
	}

	// Check if blockhashReceived == the header hash from last get headers this node performed
	// if not then increment and request next block
	if s.headerHash != block.Hash {
		nextHash, err := s.cfg.GetNextBlockHash()
		if err != nil {
			return err
		}
		err = s.cfg.RequestBlock(nextHash, block.Index)
		return err
	}

	// If we are caught up then go into normal mode
	diff := peer.Height() - block.Index
	if diff <= cruiseHeight {
		s.syncmode = normalMode
		s.timer.Reset(blockTimer)
		return nil
	}

	// If not then we go back into headersMode and request  more headers.
	s.syncmode = headersMode
	return s.cfg.RequestHeaders(block.Hash)
}

func (s *Syncmgr) blockModeOnHeaders(peer SyncPeer, hdrs []*payload.BlockBase) error {
	// We ignore headers when in this mode
	return nil
}
