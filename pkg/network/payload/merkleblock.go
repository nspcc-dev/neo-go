package payload

import (
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// MerkleBlock represents a merkle block packet payload.
type MerkleBlock struct {
	*core.BlockBase
	TxCount int
	Hashes  []util.Uint256
	Flags   []byte
}

// DecodeBinary implements Serializable interface.
func (m *MerkleBlock) DecodeBinary(br *io.BinReader) {
	m.BlockBase = &core.BlockBase{}
	m.BlockBase.DecodeBinary(br)

	m.TxCount = int(br.ReadVarUint())
	n := br.ReadVarUint()
	m.Hashes = make([]util.Uint256, n)
	for i := 0; i < len(m.Hashes); i++ {
		br.ReadLE(&m.Hashes[i])
	}
	m.Flags = br.ReadBytes()
}

// EncodeBinary implements Serializable interface.
func (m *MerkleBlock) EncodeBinary(bw *io.BinWriter) {
	m.BlockBase = &core.BlockBase{}
	m.BlockBase.EncodeBinary(bw)

	bw.WriteVarUint(uint64(m.TxCount))
	bw.WriteVarUint(uint64(len(m.Hashes)))
	for i := 0; i < len(m.Hashes); i++ {
		bw.WriteLE(m.Hashes[i])
	}
	bw.WriteBytes(m.Flags)
}
