package core

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
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

// NewBlockFromTrimmedBytes returns a new block from trimmed data.
// This is commonly used to create a block from stored data.
// Blocks created from trimmed data will have their Trimmed field
// set to true.
func NewBlockFromTrimmedBytes(b []byte) (*Block, error) {
	block := &Block{
		Trimmed: true,
	}

	r := bytes.NewReader(b)
	if err := block.decodeHashableFields(r); err != nil {
		return block, err
	}

	br := util.NewBinReaderFromIO(r)
	var padding uint8
	br.ReadLE(&padding)
	if br.Err != nil {
		return block, br.Err
	}

	block.Script = &transaction.Witness{}
	if err := block.Script.DecodeBinary(r); err != nil {
		return block, err
	}

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
	buf := new(bytes.Buffer)
	if err := b.encodeHashableFields(buf); err != nil {
		return nil, err
	}
	bw := util.NewBinWriterFromIO(buf)
	bw.WriteLE(uint8(1))
	if bw.Err != nil {
		return nil, bw.Err
	}
	if err := b.Script.EncodeBinary(buf); err != nil {
		return nil, err
	}

	bw.WriteVarUint(uint64(len(b.Transactions)))
	for _, tx := range b.Transactions {
		bw.WriteLE(tx.Hash())
	}
	if bw.Err != nil {
		return nil, bw.Err
	}
	return buf.Bytes(), nil
}

// DecodeBinary decodes the block from the given reader.
func (b *Block) DecodeBinary(r io.Reader) error {
	if err := b.BlockBase.DecodeBinary(r); err != nil {
		return err
	}

	br := util.NewBinReaderFromIO(r)
	lentx := br.ReadVarUint()
	if br.Err != nil {
		return br.Err
	}
	b.Transactions = make([]*transaction.Transaction, lentx)
	for i := 0; i < int(lentx); i++ {
		b.Transactions[i] = &transaction.Transaction{}
		if err := b.Transactions[i].DecodeBinary(r); err != nil {
			return err
		}
	}

	return nil
}

// EncodeBinary encodes the block to the given writer.
func (b *Block) EncodeBinary(w io.Writer) error {
	err := b.BlockBase.EncodeBinary(w)
	if err != nil {
		return err
	}
	bw := util.NewBinWriterFromIO(w)
	bw.WriteVarUint(uint64(len(b.Transactions)))
	if bw.Err != nil {
		return err
	}
	for _, tx := range b.Transactions {
		err := tx.EncodeBinary(w)
		if err != nil {
			return err
		}
	}
	return nil
}
