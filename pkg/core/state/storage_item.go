package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// StorageItem is the value to be stored with read-only flag.
type StorageItem struct {
	Value []byte
}

// EncodeBinary implements Serializable interface.
func (si *StorageItem) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(si.Value)
}

// DecodeBinary implements Serializable interface.
func (si *StorageItem) DecodeBinary(r *io.BinReader) {
	si.Value = r.ReadVarBytes()
}
