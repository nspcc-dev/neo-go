package payload

import (
	"math/bits"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MerkleBlock represents a merkle block packet payload.
type MerkleBlock struct {
	*block.Header
	TxCount int
	Hashes  []util.Uint256
	Flags   []byte
}

// DecodeBinary implements the Serializable interface.
func (m *MerkleBlock) DecodeBinary(br *io.BinReader) {
	m.Header = &block.Header{}
	m.Header.DecodeBinary(br)

	txCount := br.ReadVarUint()
	if txCount > block.MaxTransactionsPerBlock {
		br.Err = block.ErrMaxContentsPerBlock
		return
	}
	m.TxCount = int(txCount)
	maxCnt := roundToNextPowerOfTwo(uint(txCount))
	br.ReadArray(&m.Hashes, int(maxCnt))
	m.Flags = br.ReadVarBytes((int(txCount) + 7) / 8)
}

// EncodeBinary implements the Serializable interface.
func (m *MerkleBlock) EncodeBinary(bw *io.BinWriter) {
	m.Header.EncodeBinary(bw)

	bw.WriteVarUint(uint64(m.TxCount))
	bw.WriteArray(m.Hashes)
	bw.WriteVarBytes(m.Flags)
}

// roundToNextPowerOfTwo returns the smallest power of two that's greater than
// or equal to value. Returns 0 for zero input.
func roundToNextPowerOfTwo(n uint) uint {
	return 1 << bits.Len(n-1)
}
