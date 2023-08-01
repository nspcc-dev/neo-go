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
	c.Hash.DecodeBinary(br)
}

// EncodeBinary implements the io.Serializable interface.
func (c *Conflicts) EncodeBinary(w *io.BinWriter) {
	c.Hash.EncodeBinary(w)
}

func (c *Conflicts) toJSONMap(m map[string]any) {
	m["hash"] = c.Hash
}
