package result

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// LedgerAux is a set of methods needed to construct some outputs.
	LedgerAux interface {
		BlockHeight() uint32
		GetHeaderHash(int) util.Uint256
	}
	// Block wrapper used for the representation of
	// block.Block / block.Base on the RPC Server.
	Block struct {
		block.Block
		BlockMetadata
	}

	// BlockMetadata is an additional metadata added to standard
	// block.Block.
	BlockMetadata struct {
		Size          int           `json:"size"`
		NextBlockHash *util.Uint256 `json:"nextblockhash,omitempty"`
		Confirmations uint32        `json:"confirmations"`
	}
)

// NewBlock creates a new Block wrapper.
func NewBlock(b *block.Block, chain LedgerAux) Block {
	res := Block{
		Block: *b,
		BlockMetadata: BlockMetadata{
			Size:          io.GetVarSize(b),
			Confirmations: chain.BlockHeight() - b.Index + 1,
		},
	}

	hash := chain.GetHeaderHash(int(b.Index) + 1)
	if !hash.Equals(util.Uint256{}) {
		res.NextBlockHash = &hash
	}

	return res
}

// MarshalJSON implements json.Marshaler interface.
func (b Block) MarshalJSON() ([]byte, error) {
	output, err := json.Marshal(b.BlockMetadata)
	if err != nil {
		return nil, err
	}
	baseBytes, err := json.Marshal(b.Block)
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
	// As block.Block and BlockMetadata are at the same level in json,
	// do unmarshalling separately for both structs.
	meta := new(BlockMetadata)
	err := json.Unmarshal(data, meta)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &b.Block)
	if err != nil {
		return err
	}
	b.BlockMetadata = *meta
	return nil
}
