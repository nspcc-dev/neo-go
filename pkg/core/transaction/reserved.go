package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Reserved represents an attribute for experimental or private usage.
type Reserved struct {
	Value []byte
}

// DecodeBinary implements the io.Serializable interface.
func (e *Reserved) DecodeBinary(br *io.BinReader) {
	e.Value = br.ReadVarBytes()
}

// EncodeBinary implements the io.Serializable interface.
func (e *Reserved) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(e.Value)
}

func (e *Reserved) toJSONMap(m map[string]any) {
	m["value"] = e.Value
}

// Copy implements the AttrValue interface.
func (e *Reserved) Copy() AttrValue {
	return &Reserved{
		Value: e.Value,
	}
}
