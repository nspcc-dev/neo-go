package transaction

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Witness contains 2 scripts.
type Witness struct {
	InvocationScript   []byte
	VerificationScript []byte
}

// DecodeBinary implements the payload interface.
func (w *Witness) DecodeBinary(r io.Reader) error {
	lenb := util.ReadVarUint(r)
	w.InvocationScript = make([]byte, lenb)
	if err := binary.Read(r, binary.LittleEndian, w.InvocationScript); err != nil {
		return err
	}
	lenb = util.ReadVarUint(r)
	w.VerificationScript = make([]byte, lenb)
	return binary.Read(r, binary.LittleEndian, w.VerificationScript)
}

// EncodeBinary implements the payload interface.
func (w *Witness) EncodeBinary(writer io.Writer) error {
	if err := util.WriteVarUint(writer, uint64(len(w.InvocationScript))); err != nil {
		return err
	}
	if err := binary.Write(writer, binary.LittleEndian, w.InvocationScript); err != nil {
		return err
	}
	if err := util.WriteVarUint(writer, uint64(len(w.VerificationScript))); err != nil {
		return err
	}
	return binary.Write(writer, binary.LittleEndian, w.VerificationScript)
}

// MarshalJSON implements the json marshaller interface.
func (w *Witness) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{
		"invocation":   hex.EncodeToString(w.InvocationScript),
		"verification": hex.EncodeToString(w.VerificationScript),
	}

	return json.Marshal(data)
}
