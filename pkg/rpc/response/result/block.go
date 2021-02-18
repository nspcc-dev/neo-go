package result

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
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
		*block.Base
		BlockMetadataAndTx
	}

	// BlockMetadataAndTx is an additional metadata added to standard
	// block.Base plus specially encoded transactions.
	BlockMetadataAndTx struct {
		Size          int           `json:"size"`
		NextBlockHash *util.Uint256 `json:"nextblockhash,omitempty"`
		Confirmations uint32        `json:"confirmations"`
		Tx            []Tx          `json:"tx"`
	}
)

// NewBlock creates a new Block wrapper.
func NewBlock(b *block.Block, chain core.Blockchainer) Block {
	res := Block{
		Base: &b.Base,
		BlockMetadataAndTx: BlockMetadataAndTx{
			Size:          io.GetVarSize(b),
			Confirmations: chain.BlockHeight() - b.Index + 1,
			Tx:            make([]Tx, 0, len(b.Transactions)),
		},
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

// MarshalJSON implements json.Marshaler interface.
func (b Block) MarshalJSON() ([]byte, error) {
	output, err := json.Marshal(b.BlockMetadataAndTx)
	if err != nil {
		return nil, err
	}
	baseBytes, err := json.Marshal(b.Base)
	if err != nil {
		return nil, err
	}

	// We have to keep both "fields" at the same level in json in order to
	// match C# API, so there's no way to marshall Block correctly with
	// standard json.Marshaller tool.
	if output[len(output)-1] != '}' || baseBytes[0] != '{' {
		return nil, errors.New("can't merge internal jsons")
	}
	output[len(output)-1] = ','
	output = append(output, baseBytes[1:]...)
	return output, nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (b *Block) UnmarshalJSON(data []byte) error {
	// As block.Base and BlockMetadataAndTx are at the same level in json,
	// do unmarshalling separately for both structs.
	metaTx := new(BlockMetadataAndTx)
	base := new(block.Base)
	err := json.Unmarshal(data, metaTx)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, base)
	if err != nil {
		return err
	}
	b.Base = base
	b.BlockMetadataAndTx = *metaTx
	return nil
}
