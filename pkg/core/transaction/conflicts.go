package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Conflicts represents attribute for conflicting transactions.
type Conflicts struct {
	Hash util.Uint256 `json:"hash"`
}

// DecodeBinary implements the io.Serializable interface.
func (c *Conflicts) DecodeBinary(br *io.BinReader) {
	bytes := br.ReadVarBytes(util.Uint256Size)
	if br.Err != nil {
		return
	}
	hash, err := util.Uint256DecodeBytesBE(bytes)
	if err != nil {
		br.Err = err
		return
	}
	c.Hash = hash
}

// EncodeBinary implements the io.Serializable interface.
func (c *Conflicts) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(c.Hash.BytesBE())
}

func (c *Conflicts) toJSONMap(m map[string]interface{}) {
	m["hash"] = c.Hash
}
