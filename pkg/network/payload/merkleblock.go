package payload

import (
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MerkleBlock represents a merkle block packet payload.
type MerkleBlock struct {
	*block.Base
	TxCount int
	Hashes  []util.Uint256
	Flags   []byte
}

// DecodeBinary implements Serializable interface.
func (m *MerkleBlock) DecodeBinary(br *io.BinReader) {
	m.Base = &block.Base{}
	m.Base.DecodeBinary(br)

	m.TxCount = int(br.ReadVarUint())
	br.ReadArray(&m.Hashes)
	m.Flags = br.ReadVarBytes()
}

// EncodeBinary implements Serializable interface.
func (m *MerkleBlock) EncodeBinary(bw *io.BinWriter) {
	m.Base.EncodeBinary(bw)

	bw.WriteVarUint(uint64(m.TxCount))
	bw.WriteArray(m.Hashes)
	bw.WriteVarBytes(m.Flags)
}
