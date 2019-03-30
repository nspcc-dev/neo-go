package server

import (
	"encoding/binary"

	"github.com/CityOfZion/neo-go/pkg/peermgr"

	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/syncmgr"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

func setupSyncManager(s *Server) *syncmgr.Syncmgr {

	cfg := &syncmgr.Config{
		ProcessBlock:   s.processBlock,
		ProcessHeaders: s.processHeaders,

		RequestBlock:   s.requestBlock,
		RequestHeaders: s.requestHeaders,

		GetNextBlockHash: s.getNextBlockHash,
		AskForNewBlocks:  s.askForNewBlocks,

		FetchHeadersAgain: s.fetchHeadersAgain,
		FetchBlockAgain:   s.fetchBlockAgain,
	}

	return syncmgr.New(cfg)
}

func (s *Server) onHeader(peer *peer.Peer, hdrsMessage *payload.HeadersMessage) {
	s.pmg.MsgReceived(peer, hdrsMessage.Command())
	s.smg.OnHeader(peer, hdrsMessage)
}

func (s *Server) onBlock(peer *peer.Peer, blockMsg *payload.BlockMessage) {
	s.pmg.BlockMsgReceived(peer, peermgr.BlockInfo{
		BlockHash:  blockMsg.Hash,
		BlockIndex: uint64(blockMsg.Index),
	})
	s.smg.OnBlock(peer, blockMsg)
}

func (s *Server) processBlock(block payload.Block) error {
	return s.chain.ProcessBlock(block)
}

func (s *Server) processHeaders(hdrs []*payload.BlockBase) error {
	return s.chain.ProcessHeaders(hdrs)
}

func (s *Server) requestHeaders(hash util.Uint256) error {
	return s.pmg.RequestHeaders(hash)
}

func (s *Server) requestBlock(hash util.Uint256) error {
	return s.pmg.RequestBlock(hash)
}

// getNextBlockHash searches the database for the blockHash
// that is the height above our best block. The hash will be taken from a header.
func (s *Server) getNextBlockHash() (util.Uint256, error) {
	bestBlock, err := s.chain.Db.GetLastBlock()
	if err != nil {
		// Panic!
		// XXX: One alternative, is to get the network, erase the database and then start again from scratch.
		// This should never happen. The latest block will always be atleast the genesis block
		panic("could not get best block from database" + err.Error())
	}

	index := make([]byte, 4)
	binary.BigEndian.PutUint32(index, bestBlock.Index+1)

	hdr, err := s.chain.Db.GetHeaderFromHeight(index)
	if err != nil {
		return util.Uint256{}, err
	}
	return hdr.Hash, nil
}

func (s *Server) getBestBlockHash() (util.Uint256, error) {
	return util.Uint256{}, nil
}

func (s *Server) askForNewBlocks() {
	// send a getblocks message with the latest block saved

	// when we receive something then send get data
}

func (s *Server) fetchHeadersAgain(util.Uint256) error {
	return nil
}

func (s *Server) fetchBlockAgain(util.Uint256) error {
	return nil
}
