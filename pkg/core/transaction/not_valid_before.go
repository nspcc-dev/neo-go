package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// NotValidBefore represents attribute with the height transaction is not valid before.
type NotValidBefore struct {
	Height uint32 `json:"height"`
}

// DecodeBinary implements the io.Serializable interface.
func (n *NotValidBefore) DecodeBinary(br *io.BinReader) {
	n.Height = br.ReadU32LE()
}

// EncodeBinary implements the io.Serializable interface.
func (n *NotValidBefore) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(n.Height)
}

func (n *NotValidBefore) toJSONMap(m map[string]any) {
	m["height"] = n.Height
}

// Copy implements the AttrValue interface.
func (n *NotValidBefore) Copy() AttrValue {
	return &NotValidBefore{
		Height: n.Height,
	}
}
