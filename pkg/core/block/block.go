package block

import (
	"errors"
	"fmt"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Block represents one block in the chain.
type Block struct {
	// The base of the block.
	Base

	// Transaction list.
	Transactions []*transaction.Transaction `json:"tx"`

	// True if this block is created from trimmed data.
	Trimmed bool `json:"-"`
}

// Header returns the Header of the Block.
func (b *Block) Header() *Header {
	return &Header{
		Base: b.Base,
	}
}

func merkleTreeFromTransactions(txes []*transaction.Transaction) (*hash.MerkleTree, error) {
	hashes := make([]util.Uint256, len(txes))
	for i, tx := range txes {
		hashes[i] = tx.Hash()
	}

	return hash.NewMerkleTree(hashes)
}

// RebuildMerkleRoot rebuilds the merkleroot of the block.
func (b *Block) RebuildMerkleRoot() error {
	merkle, err := merkleTreeFromTransactions(b.Transactions)
	if err != nil {
		return err
	}

	b.MerkleRoot = merkle.Root()
	return nil
}

// Verify verifies the integrity of the block.
func (b *Block) Verify() error {
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
			return fmt.Errorf("miner transaction %s is not the first one", tx.Hash().StringLE())
		}
	}
	merkle, err := merkleTreeFromTransactions(b.Transactions)
	if err != nil {
		return err
	}
	if !b.MerkleRoot.Equals(merkle.Root()) {
		return errors.New("MerkleRoot mismatch")
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

	_ = br.ReadB()

	block.Script.DecodeBinary(br)

	lenTX := br.ReadVarUint()
	block.Transactions = make([]*transaction.Transaction, lenTX)
	for i := 0; i < int(lenTX); i++ {
		var hash util.Uint256
		hash.DecodeBinary(br)
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
	buf.WriteB(1)
	b.Script.EncodeBinary(buf.BinWriter)

	buf.WriteVarUint(uint64(len(b.Transactions)))
	for _, tx := range b.Transactions {
		h := tx.Hash()
		h.EncodeBinary(buf.BinWriter)
	}
	if buf.Err != nil {
		return nil, buf.Err
	}
	return buf.Bytes(), nil
}

// DecodeBinary decodes the block from the given BinReader, implementing
// Serializable interface.
func (b *Block) DecodeBinary(br *io.BinReader) {
	b.Base.DecodeBinary(br)
	br.ReadArray(&b.Transactions)
}

// EncodeBinary encodes the block to the given BinWriter, implementing
// Serializable interface.
func (b *Block) EncodeBinary(bw *io.BinWriter) {
	b.Base.EncodeBinary(bw)
	bw.WriteArray(b.Transactions)
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
