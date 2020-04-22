package block

import (
	"errors"

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

	// Primary index and nonce
	ConsensusData ConsensusData `json:"consensus_data"`

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

// computeMerkleTree computes Merkle tree based on actual block's data.
func (b *Block) computeMerkleTree() (*hash.MerkleTree, error) {
	hashes := make([]util.Uint256, len(b.Transactions)+1)
	hashes[0] = b.ConsensusData.Hash()
	for i, tx := range b.Transactions {
		hashes[i+1] = tx.Hash()
	}

	return hash.NewMerkleTree(hashes)
}

// RebuildMerkleRoot rebuilds the merkleroot of the block.
func (b *Block) RebuildMerkleRoot() error {
	merkle, err := b.computeMerkleTree()
	if err != nil {
		return err
	}

	b.MerkleRoot = merkle.Root()
	return nil
}

// Verify verifies the integrity of the block.
func (b *Block) Verify() error {
	merkle, err := b.computeMerkleTree()
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

	lenHashes := br.ReadVarUint()
	if lenHashes > 0 {
		var consensusDataHash util.Uint256
		consensusDataHash.DecodeBinary(br)
		lenTX := lenHashes - 1
		block.Transactions = make([]*transaction.Transaction, lenTX)
		for i := 0; i < int(lenTX); i++ {
			var hash util.Uint256
			hash.DecodeBinary(br)
			block.Transactions[i] = transaction.NewTrimmedTX(hash)
		}
		block.ConsensusData.DecodeBinary(br)
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

	buf.WriteVarUint(uint64(len(b.Transactions)) + 1)
	hash := b.ConsensusData.Hash()
	hash.EncodeBinary(buf.BinWriter)

	for _, tx := range b.Transactions {
		h := tx.Hash()
		h.EncodeBinary(buf.BinWriter)
	}

	b.ConsensusData.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, buf.Err
	}

	return buf.Bytes(), nil
}

// DecodeBinary decodes the block from the given BinReader, implementing
// Serializable interface.
func (b *Block) DecodeBinary(br *io.BinReader) {
	b.Base.DecodeBinary(br)
	contentsCount := br.ReadVarUint()
	if contentsCount == 0 {
		br.Err = errors.New("invalid block format")
		return
	}
	b.ConsensusData.DecodeBinary(br)
	txes := make([]*transaction.Transaction, contentsCount-1)
	for i := 0; i < int(contentsCount)-1; i++ {
		tx := new(transaction.Transaction)
		tx.DecodeBinary(br)
		txes[i] = tx
	}
	b.Transactions = txes
	merkle, err := b.computeMerkleTree()
	if err != nil {
		br.Err = err
		return
	}
	if !b.MerkleRoot.Equals(merkle.Root()) {
		br.Err = errors.New("MerkleRoot mismatch")
		return
	}
}

// EncodeBinary encodes the block to the given BinWriter, implementing
// Serializable interface.
func (b *Block) EncodeBinary(bw *io.BinWriter) {
	b.Base.EncodeBinary(bw)
	bw.WriteVarUint(uint64(len(b.Transactions) + 1))
	b.ConsensusData.EncodeBinary(bw)
	for i := 0; i < len(b.Transactions); i++ {
		b.Transactions[i].EncodeBinary(bw)
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
