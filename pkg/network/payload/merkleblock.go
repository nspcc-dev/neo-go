package payload

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/util"
)

type MerkleBlock struct {
	*core.BlockBase
	TxCount int
	Hashes  []util.Uint256
	Flags   []byte
}

func (m *MerkleBlock) DecodeBinary(r io.Reader) error {
	m.BlockBase = &core.BlockBase{}
	if err := m.BlockBase.DecodeBinary(r); err != nil {
		return err
	}

	m.TxCount = int(util.ReadVarUint(r))
	n := util.ReadVarUint(r)
	m.Hashes = make([]util.Uint256, n)
	for i := 0; i < len(m.Hashes); i++ {
		if err := binary.Read(r, binary.LittleEndian, &m.Hashes[i]); err != nil {
			return err
		}
	}
	var err error
	m.Flags, err = util.ReadVarBytes(r)
	return err
}

func (m *MerkleBlock) EncodeBinary(w io.Writer) error {
	return nil
}
