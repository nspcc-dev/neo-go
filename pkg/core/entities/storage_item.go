package entities

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// StorageItem is the value to be stored with read-only flag.
type StorageItem struct {
	Value   []byte
	IsConst bool
}

// EncodeBinary implements Serializable interface.
func (si *StorageItem) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(si.Value)
	w.WriteLE(si.IsConst)
}

// DecodeBinary implements Serializable interface.
func (si *StorageItem) DecodeBinary(r *io.BinReader) {
	si.Value = r.ReadVarBytes()
	r.ReadLE(&si.IsConst)
}
