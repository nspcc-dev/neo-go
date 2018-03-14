package core

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

// Header holds the head info of a block.
type Header struct {
	// Base of the block.
	BlockBase
	// Padding that is fixed to 0
	_ uint8
}

// DecodeBinary impelements the Payload interface.
func (h *Header) DecodeBinary(r io.Reader) error {
	if err := h.BlockBase.DecodeBinary(r); err != nil {
		return err
	}

	var padding uint8
	binary.Read(r, binary.LittleEndian, &padding)
	if padding != 0 {
		return fmt.Errorf("format error: padding must equal 0 got %d", padding)
	}

	return nil
}

// EncodeBinary  impelements the Payload interface.
func (h *Header) EncodeBinary(w io.Writer) error {
	if err := h.BlockBase.EncodeBinary(w); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, uint8(0))
}

// Block represents one block in the chain.
type Block struct {
	// The base of the block.
	BlockBase
	// Transaction list.
	Transactions []*transaction.Transaction
}

// Header returns a pointer to the head of the block (BlockHead).
func (b *Block) Header() *Header {
	return &Header{
		BlockBase: b.BlockBase,
	}
}

// Verify the integrity of the block.
func (b *Block) Verify(full bool) bool {
	// The first TX has to be a miner transaction.
	if b.Transactions[0].Type != transaction.MinerType {
		return false
	}

	// If the first TX is a minerTX then all others cant.
	for _, tx := range b.Transactions[1:] {
		if tx.Type == transaction.MinerType {
			return false
		}
	}

	// TODO: When full is true, do a full verification.
	if full {
		log.Warn("full verification of blocks is not yet implemented")
	}

	return true
}

// EncodeBinary encodes the block to the given writer.
func (b *Block) EncodeBinary(w io.Writer) error {
	return nil
}

// DecodeBinary decodes the block from the given reader.
func (b *Block) DecodeBinary(r io.Reader) error {
	if err := b.BlockBase.DecodeBinary(r); err != nil {
		return err
	}

	lentx := util.ReadVarUint(r)
	b.Transactions = make([]*transaction.Transaction, lentx)
	for i := 0; i < int(lentx); i++ {
		b.Transactions[i] = &transaction.Transaction{}
		if err := b.Transactions[i].DecodeBinary(r); err != nil {
			return err
		}
	}

	return nil
}
