package transaction

import (
	"encoding/hex"
	"encoding/json"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Witness contains 2 scripts.
type Witness struct {
	InvocationScript   []byte
	VerificationScript []byte
}

// DecodeBinary implements the payload interface.
func (w *Witness) DecodeBinary(br *io.BinReader) error {
	w.InvocationScript = br.ReadBytes()
	w.VerificationScript = br.ReadBytes()
	return br.Err
}

// EncodeBinary implements the payload interface.
func (w *Witness) EncodeBinary(bw *io.BinWriter) error {
	bw.WriteBytes(w.InvocationScript)
	bw.WriteBytes(w.VerificationScript)

	return bw.Err
}

// MarshalJSON implements the json marshaller interface.
func (w *Witness) MarshalJSON() ([]byte, error) {
	data := map[string]string{
		"invocation":   hex.EncodeToString(w.InvocationScript),
		"verification": hex.EncodeToString(w.VerificationScript),
	}

	return json.Marshal(data)
}

// ScriptHash returns the hash of the VerificationScript.
func (w Witness) ScriptHash() util.Uint160 {
	return hash.Hash160(w.VerificationScript)
}
