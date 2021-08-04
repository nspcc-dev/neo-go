package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// MaxInvocationScript is the maximum length of allowed invocation
	// script. It should fit 11/21 multisignature for the committee.
	MaxInvocationScript = 1024

	// MaxVerificationScript is the maximum allowed length of verification
	// script. It should be appropriate for 11/21 multisignature committee.
	MaxVerificationScript = 1024
)

// Witness contains 2 scripts.
type Witness struct {
	InvocationScript   []byte `json:"invocation"`
	VerificationScript []byte `json:"verification"`
}

// DecodeBinary implements Serializable interface.
func (w *Witness) DecodeBinary(br io.BinaryReader) {
	w.InvocationScript = br.ReadVarBytes(MaxInvocationScript)
	w.VerificationScript = br.ReadVarBytes(MaxVerificationScript)
}

// EncodeBinary implements Serializable interface.
func (w *Witness) EncodeBinary(bw io.BinaryWriter) {
	bw.WriteVarBytes(w.InvocationScript)
	bw.WriteVarBytes(w.VerificationScript)
}

// ScriptHash returns the hash of the VerificationScript.
func (w Witness) ScriptHash() util.Uint160 {
	return hash.Hash160(w.VerificationScript)
}
