package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Conflicts represents attribute for conflicting transactions.
type Conflicts struct {
	Hash util.Uint256 `json:"hash"`
}

// DecodeBinary implements io.Serializable interface.
func (c *Conflicts) DecodeBinary(br io.BinaryReader) {
	bytes := br.ReadVarBytes(util.Uint256Size)
	if br.Error() != nil {
		return
	}
	hash, err := util.Uint256DecodeBytesBE(bytes)
	if err != nil {
		br.SetError(err)
		return
	}
	c.Hash = hash
}

// EncodeBinary implements io.Serializable interface.
func (c *Conflicts) EncodeBinary(w io.BinaryWriter) {
	w.WriteVarBytes(c.Hash.BytesBE())
}

func (c *Conflicts) toJSONMap(m map[string]interface{}) {
	m["hash"] = c.Hash
}
