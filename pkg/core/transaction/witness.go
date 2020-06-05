package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Witness contains 2 scripts.
type Witness struct {
	InvocationScript   []byte `json:"invocation"`
	VerificationScript []byte `json:"verification"`
}

// DecodeBinary implements Serializable interface.
func (w *Witness) DecodeBinary(br *io.BinReader) {
	w.InvocationScript = br.ReadVarBytes()
	w.VerificationScript = br.ReadVarBytes()
}

// EncodeBinary implements Serializable interface.
func (w *Witness) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarBytes(w.InvocationScript)
	bw.WriteVarBytes(w.VerificationScript)
}

// ScriptHash returns the hash of the VerificationScript.
func (w Witness) ScriptHash() util.Uint160 {
	return hash.Hash160(w.VerificationScript)
}
