package syncmgr

import (
	"github.com/CityOfZion/neo-go/pkg/chain"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

// blockModeOnBlock is called when the sync manager is block mode
// and receives a block.
func (s *Syncmgr) blockModeOnBlock(peer SyncPeer, block payload.Block) error {

	// Check if it is a future block
	// XXX: since we are storing blocks in memory, we do not want to store blocks
	// from the tip
	if block.Index > s.nextBlockIndex+2000 {
		return nil
	}
	if block.Index > s.nextBlockIndex {
		s.addToBlockPool(block)
		return nil
	}

	// Process Block
	err := s.processBlock(block)
	if err != nil && err != chain.ErrBlockAlreadyExists {
		return s.cfg.FetchBlockAgain(block.Hash)
	}

	// Check the block pool
	err = s.checkPool()
	if err != nil {
		return err
	}

	// Check if blockhashReceived == the header hash from last get headers this node performed
	// if not then increment and request next block
	if s.headerHash != block.Hash {
		nextHash, err := s.cfg.GetNextBlockHash()
		if err != nil {
			return err
		}
		return s.cfg.RequestBlock(nextHash, block.Index)
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
