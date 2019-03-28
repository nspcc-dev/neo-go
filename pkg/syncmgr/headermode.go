package syncmgr

import (
	"github.com/CityOfZion/neo-go/pkg/chain"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

// headersModeOnHeaders is called when the sync manager is headers mode
// and receives a header.
func (s *Syncmgr) headersModeOnHeaders(peer SyncPeer, hdrs []*payload.BlockBase) error {
	// If we are in Headers mode, then we just need to process the headers
	// Note: For the un-optimised version, we move straight to blocksOnly mode

	firstHash := hdrs[0].Hash

	err := s.cfg.ProcessHeaders(hdrs)
	if err == nil {
		// Update syncmgr last header
		s.headerHash = hdrs[len(hdrs)-1].Hash

		s.syncmode = blockMode
		return s.cfg.RequestBlock(firstHash)
	}

	// Check whether it is a validation error, or a database error
	if _, ok := err.(*chain.ValidationError); ok {
		// If we get a validation error we re-request the headers
		// the method will automatically fetch from a different peer
		// XXX: Add increment banScore for this peer
		return s.cfg.FetchHeadersAgain(firstHash)
	}
	// This means it is a database error. We have no way to recover from this.
	panic(err.Error())
}

// headersModeOnBlock is called when the sync manager is headers mode
// and receives a block.
func (s *Syncmgr) headersModeOnBlock(peer SyncPeer, block payload.Block) error {
	// While in headers mode, ignore any blocks received
	return nil
}
