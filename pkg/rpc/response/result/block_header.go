package result

import (
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// Header wrapper used for the representation of
	// block header on the RPC Server.
	Header struct {
		Hash          util.Uint256        `json:"hash"`
		Size          int                 `json:"size"`
		Version       uint32              `json:"version"`
		PrevBlockHash util.Uint256        `json:"previousblockhash"`
		MerkleRoot    util.Uint256        `json:"merkleroot"`
		Timestamp     uint64              `json:"time"`
		Index         uint32              `json:"index"`
		Nonce         string              `json:"nonce"`
		NextConsensus string              `json:"nextconsensus"`
		Script        transaction.Witness `json:"script"`
		Confirmations uint32              `json:"confirmations"`
		NextBlockHash *util.Uint256       `json:"nextblockhash,omitempty"`
	}
)

// NewHeader creates a new Header wrapper.
func NewHeader(h *block.Header, chain blockchainer.Blockchainer) Header {
	res := Header{
		Hash:          h.Hash(),
		Size:          io.GetVarSize(h),
		Version:       h.Version,
		PrevBlockHash: h.PrevHash,
		MerkleRoot:    h.MerkleRoot,
		Timestamp:     h.Timestamp,
		Index:         h.Index,
		Nonce:         strconv.FormatUint(h.ConsensusData, 16),
		NextConsensus: address.Uint160ToString(h.NextConsensus),
		Script:        h.Script,
		Confirmations: chain.BlockHeight() - h.Index + 1,
	}

	hash := chain.GetHeaderHash(int(h.Index) + 1)
	if !hash.Equals(util.Uint256{}) {
		res.NextBlockHash = &hash
	}
	return res
}
