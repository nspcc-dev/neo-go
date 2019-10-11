package core

import (
	"errors"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/Workiva/go-datastructures/queue"
	log "github.com/sirupsen/logrus"
)

// Block represents one block in the chain.
type Block struct {
	// The base of the block.
	BlockBase

	// Transaction list.
	Transactions []*transaction.Transaction `json:"tx"`

	// True if this block is created from trimmed data.
	Trimmed bool `json:"-"`
}

// Header returns the Header of the Block.
func (b *Block) Header() *Header {
	return &Header{
		BlockBase: b.BlockBase,
	}
}

// rebuildMerkleRoot rebuild the merkleroot of the block.
func (b *Block) rebuildMerkleRoot() error {
	hashes := make([]util.Uint256, len(b.Transactions))
	for i, tx := range b.Transactions {
		hashes[i] = tx.Hash()
	}

	merkle, err := crypto.NewMerkleTree(hashes)
	if err != nil {
		return err
	}

	b.MerkleRoot = merkle.Root()
	return nil
}

// Verify the integrity of the block.
func (b *Block) Verify(full bool) error {
	// There has to be some transaction inside.
	if len(b.Transactions) == 0 {
		return errors.New("no transactions")
	}
	// The first TX has to be a miner transaction.
	if b.Transactions[0].Type != transaction.MinerType {
		return fmt.Errorf("the first transaction is %s", b.Transactions[0].Type)
	}
	// If the first TX is a minerTX then all others cant.
	for _, tx := range b.Transactions[1:] {
		if tx.Type == transaction.MinerType {
			return fmt.Errorf("miner transaction %s is not the first one", tx.Hash().ReverseString())
		}
	}
	// TODO: When full is true, do a full verification.
	if full {
		log.Warn("full verification of blocks is not yet implemented")
	}
	return nil
}

// NewBlockFromTrimmedBytes returns a new block from trimmed data.
// This is commonly used to create a block from stored data.
// Blocks created from trimmed data will have their Trimmed field
// set to true.
func NewBlockFromTrimmedBytes(b []byte) (*Block, error) {
	block := &Block{
		Trimmed: true,
	}

	br := io.NewBinReaderFromBuf(b)
	block.decodeHashableFields(br)

	var padding uint8
	br.ReadLE(&padding)

	block.Script = &transaction.Witness{}
	block.Script.DecodeBinary(br)

	lenTX := br.ReadVarUint()
	block.Transactions = make([]*transaction.Transaction, lenTX)
	for i := 0; i < int(lenTX); i++ {
		var hash util.Uint256
		br.ReadLE(&hash)
		block.Transactions[i] = transaction.NewTrimmedTX(hash)
	}

	return block, br.Err
}

// Trim returns a subset of the block data to save up space
// in storage.
// Notice that only the hashes of the transactions are stored.
func (b *Block) Trim() ([]byte, error) {
	buf := io.NewBufBinWriter()
	b.encodeHashableFields(buf.BinWriter)
	buf.WriteLE(uint8(1))
	b.Script.EncodeBinary(buf.BinWriter)

	buf.WriteVarUint(uint64(len(b.Transactions)))
	for _, tx := range b.Transactions {
		buf.WriteLE(tx.Hash())
	}
	if buf.Err != nil {
		return nil, buf.Err
	}
	return buf.Bytes(), nil
}

// DecodeBinary decodes the block from the given BinReader, implementing
// Serializable interface.
func (b *Block) DecodeBinary(br *io.BinReader) {
	b.BlockBase.DecodeBinary(br)

	lentx := br.ReadVarUint()
	b.Transactions = make([]*transaction.Transaction, lentx)
	for i := 0; i < int(lentx); i++ {
		b.Transactions[i] = &transaction.Transaction{}
		b.Transactions[i].DecodeBinary(br)
	}
}

// EncodeBinary encodes the block to the given BinWriter, implementing
// Serializable interface.
func (b *Block) EncodeBinary(bw *io.BinWriter) {
	b.BlockBase.EncodeBinary(bw)
	bw.WriteVarUint(uint64(len(b.Transactions)))
	for _, tx := range b.Transactions {
		tx.EncodeBinary(bw)
	}
}

// Compare implements the queue Item interface.
func (b *Block) Compare(item queue.Item) int {
	other := item.(*Block)
	switch {
	case b.Index > other.Index:
		return 1
	case b.Index == other.Index:
		return 0
	default:
		return -1
	}
}
