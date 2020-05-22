package result

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// ConsensusData is a wrapper for block.ConsensusData
	ConsensusData struct {
		PrimaryIndex uint32 `json:"primary"`
		Nonce        string `json:"nonce"`
	}

	// Block wrapper used for the representation of
	// block.Block / block.Base on the RPC Server.
	Block struct {
		Hash              util.Uint256  `json:"hash"`
		Size              int           `json:"size"`
		Version           uint32        `json:"version"`
		NextBlockHash     *util.Uint256 `json:"nextblockhash,omitempty"`
		PreviousBlockHash util.Uint256  `json:"previousblockhash"`
		MerkleRoot        util.Uint256  `json:"merkleroot"`
		Time              uint64        `json:"time"`
		Index             uint32        `json:"index"`
		ConsensusData     ConsensusData `json:"consensus_data"`
		NextConsensus     string        `json:"nextconsensus"`

		Confirmations uint32 `json:"confirmations"`

		Script transaction.Witness `json:"script"`

		Tx []*transaction.Transaction `json:"tx"`
	}
)

// NewBlock creates a new Block wrapper.
func NewBlock(b *block.Block, chain blockchainer.Blockchainer) Block {
	res := Block{
		Version:           b.Version,
		Hash:              b.Hash(),
		Size:              io.GetVarSize(b),
		PreviousBlockHash: b.PrevHash,
		MerkleRoot:        b.MerkleRoot,
		Time:              b.Timestamp,
		Index:             b.Index,
		ConsensusData: ConsensusData{
			PrimaryIndex: b.ConsensusData.PrimaryIndex,
			Nonce:        fmt.Sprintf("%016x", b.ConsensusData.Nonce),
		},
		NextConsensus: address.Uint160ToString(b.NextConsensus),
		Confirmations: chain.BlockHeight() - b.Index - 1,

		Script: b.Script,

		Tx: b.Transactions,
	}

	hash := chain.GetHeaderHash(int(b.Index) + 1)
	if !hash.Equals(util.Uint256{}) {
		res.NextBlockHash = &hash
	}

	return res
}
