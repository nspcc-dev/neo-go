package result

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// Tx wrapper used for the representation of
	// transaction on the RPC Server.
	Tx struct {
		TxID       util.Uint256            `json:"txid"`
		Size       int                     `json:"size"`
		Type       transaction.TXType      `json:"type"`
		Version    uint8                   `json:"version"`
		Attributes []transaction.Attribute `json:"attributes"`
		VIn        []transaction.Input     `json:"vin"`
		VOut       []transaction.Output    `json:"vout"`
		Scripts    []transaction.Witness   `json:"scripts"`

		SysFee util.Fixed8 `json:"sys_fee"`
		NetFee util.Fixed8 `json:"net_fee"`

		Nonce uint32 `json:"nonce,omitempty"`
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
		Time              uint32        `json:"time"`
		Index             uint32        `json:"index"`
		Nonce             string        `json:"nonce"`
		NextConsensus     string        `json:"nextconsensus"`

		Confirmations uint32 `json:"confirmations"`

		Script transaction.Witness `json:"script"`

		Tx []Tx `json:"tx"`
	}
)

// NewBlock creates a new Block wrapper.
func NewBlock(b *block.Block, chain core.Blockchainer) Block {
	res := Block{
		Version:           b.Version,
		Hash:              b.Hash(),
		Size:              io.GetVarSize(b),
		PreviousBlockHash: b.PrevHash,
		MerkleRoot:        b.MerkleRoot,
		Time:              b.Timestamp,
		Index:             b.Index,
		Nonce:             fmt.Sprintf("%016x", b.ConsensusData),
		NextConsensus:     address.Uint160ToString(b.NextConsensus),
		Confirmations:     chain.BlockHeight() - b.Index - 1,

		Script: b.Script,

		Tx: make([]Tx, 0, len(b.Transactions)),
	}

	hash := chain.GetHeaderHash(int(b.Index) + 1)
	if !hash.Equals(util.Uint256{}) {
		res.NextBlockHash = &hash
	}

	for i := range b.Transactions {
		tx := Tx{
			TxID:       b.Transactions[i].Hash(),
			Size:       io.GetVarSize(b.Transactions[i]),
			Type:       b.Transactions[i].Type,
			Version:    b.Transactions[i].Version,
			Attributes: make([]transaction.Attribute, 0, len(b.Transactions[i].Attributes)),
			VIn:        make([]transaction.Input, 0, len(b.Transactions[i].Inputs)),
			VOut:       make([]transaction.Output, 0, len(b.Transactions[i].Outputs)),
			Scripts:    make([]transaction.Witness, 0, len(b.Transactions[i].Scripts)),
		}

		copy(tx.Attributes, b.Transactions[i].Attributes)
		copy(tx.VIn, b.Transactions[i].Inputs)
		copy(tx.VOut, b.Transactions[i].Outputs)
		copy(tx.Scripts, b.Transactions[i].Scripts)

		tx.SysFee = chain.SystemFee(b.Transactions[i])
		tx.NetFee = chain.NetworkFee(b.Transactions[i])

		// set nonce only for MinerTransaction
		if miner, ok := b.Transactions[i].Data.(*transaction.MinerTX); ok {
			tx.Nonce = miner.Nonce
		}

		res.Tx = append(res.Tx, tx)
	}

	return res
}
