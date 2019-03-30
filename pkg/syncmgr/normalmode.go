package syncmgr

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

func (s *Syncmgr) normalModeOnHeaders(peer SyncPeer, hdrs []*payload.BlockBase) error {
	// If in normal mode, first process the headers
	err := s.cfg.ProcessHeaders(hdrs)
	if err != nil {
		// If something went wrong with processing the headers
		// Ask another peer for the headers.
		//XXX: Increment banscore for this peer
		return s.cfg.FetchHeadersAgain(hdrs[0].Hash)
	}

	lenHeaders := len(hdrs)
	firstHash := hdrs[0].Hash
	firstHdrIndex := hdrs[0].Index
	lastHash := hdrs[lenHeaders-1].Hash

	// Update syncmgr latest header
	s.headerHash = lastHash

	// If there are 2k headers, then ask for more headers and switch back to headers mode.
	if lenHeaders == 2000 {
		s.syncmode = headersMode
		return s.cfg.RequestHeaders(lastHash)
	}

	// Ask for the corresponding block iff there is < 2k headers
	// then switch to blocksMode
	// Bounds state that len > 1 && len!= 2000 & maxHeadersInMessage == 2000
	// This means that we have less than 2k headers
	s.syncmode = blockMode
	return s.cfg.RequestBlock(firstHash, firstHdrIndex)
}

// normalModeOnBlock is called when the sync manager is normal mode
// and receives a block.
func (s *Syncmgr) normalModeOnBlock(peer SyncPeer, block payload.Block) error {
	// stop the timer that periodically asks for blocks
	s.timer.Stop()

	// process block
	err := s.cfg.ProcessBlock(block)
	if err != nil {
		s.timer.Reset(blockTimer)
		return s.cfg.FetchBlockAgain(block.Hash)
	}

	diff := peer.Height() - block.Index
	if diff > trailingHeight {
		s.syncmode = headersMode
		return s.cfg.RequestHeaders(block.Hash)
	}

	s.timer.Reset(blockTimer)
	return nil
}
