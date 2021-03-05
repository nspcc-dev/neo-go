package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// StorageItem is the value to be stored with read-only flag.
type StorageItem []byte

// EncodeBinary implements Serializable interface.
func (si *StorageItem) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(*si)
}

// DecodeBinary implements Serializable interface.
func (si *StorageItem) DecodeBinary(r *io.BinReader) {
	*si = r.ReadVarBytes()
}
