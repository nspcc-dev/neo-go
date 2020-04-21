package result

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// Tx wrapper used for the representation of
	// transaction on the RPC Server.
	Tx struct {
		*transaction.Transaction
		Fees
	}

	// Fees is an auxilliary struct for proper Tx marshaling.
	Fees struct {
		SysFee util.Fixed8 `json:"sys_fee"`
		NetFee util.Fixed8 `json:"net_fee"`
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
		Nonce             string        `json:"nonce"`
		NextConsensus     string        `json:"nextconsensus"`

		Confirmations uint32 `json:"confirmations"`

		Script transaction.Witness `json:"script"`

		Tx []Tx `json:"tx"`
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
		res.Tx = append(res.Tx, Tx{
			Transaction: b.Transactions[i],
			Fees: Fees{
				SysFee: chain.SystemFee(b.Transactions[i]),
				NetFee: chain.NetworkFee(b.Transactions[i]),
			},
		})
	}

	return res
}

// MarshalJSON implements json.Marshaler interface.
func (t Tx) MarshalJSON() ([]byte, error) {
	output, err := json.Marshal(&Fees{
		SysFee: t.SysFee,
		NetFee: t.NetFee,
	})
	if err != nil {
		return nil, err
	}
	txBytes, err := json.Marshal(t.Transaction)
	if err != nil {
		return nil, err
	}

	// We have to keep both transaction.Transaction and tx at the same level in json in order to match C# API,
	// so there's no way to marshall Tx correctly with standard json.Marshaller tool.
	if output[len(output)-1] != '}' || txBytes[0] != '{' {
		return nil, errors.New("can't merge internal jsons")
	}
	output[len(output)-1] = ','
	output = append(output, txBytes[1:]...)
	return output, nil
}

// UnmarshalJSON implements json.Marshaler interface.
func (t *Tx) UnmarshalJSON(data []byte) error {
	// As transaction.Transaction and tx are at the same level in json, do unmarshalling
	// separately for both structs.
	output := new(Fees)
	err := json.Unmarshal(data, output)
	if err != nil {
		return err
	}
	t.SysFee = output.SysFee
	t.NetFee = output.NetFee

	transaction := new(transaction.Transaction)
	err = json.Unmarshal(data, transaction)
	if err != nil {
		return err
	}
	t.Transaction = transaction
	return nil
}
