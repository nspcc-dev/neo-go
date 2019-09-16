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

// DecodeBinary implements the Payload interface.
func (m *MerkleBlock) DecodeBinary(br *io.BinReader) error {
	m.BlockBase = &core.BlockBase{}
	if err := m.BlockBase.DecodeBinary(br); err != nil {
		return err
	}

	m.TxCount = int(br.ReadVarUint())
	n := br.ReadVarUint()
	m.Hashes = make([]util.Uint256, n)
	for i := 0; i < len(m.Hashes); i++ {
		br.ReadLE(&m.Hashes[i])
	}
	m.Flags = br.ReadBytes()
	return br.Err
}

// EncodeBinary implements the Payload interface.
func (m *MerkleBlock) EncodeBinary(bw *io.BinWriter) error {
	return nil
}
