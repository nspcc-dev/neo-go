package payload

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// MPTData represents the set of serialized MPT nodes.
type MPTData struct {
	Nodes [][]byte
}

// EncodeBinary implements io.Serializable.
func (d *MPTData) EncodeBinary(w *io.BinWriter) {
	w.WriteVarUint(uint64(len(d.Nodes)))
	for _, n := range d.Nodes {
		w.WriteVarBytes(n)
	}
}

// DecodeBinary implements io.Serializable.
func (d *MPTData) DecodeBinary(r *io.BinReader) {
	sz := r.ReadVarUint()
	if sz == 0 {
		r.Err = errors.New("empty MPT nodes list")
		return
	}
	for i := uint64(0); i < sz; i++ {
		d.Nodes = append(d.Nodes, r.ReadVarBytes())
		if r.Err != nil {
			return
		}
	}
}
