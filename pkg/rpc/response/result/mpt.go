package result

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// StateHeight is a result of getstateheight RPC.
type StateHeight struct {
	BlockHeight uint32 `json:"blockHeight"`
	StateHeight uint32 `json:"stateHeight"`
}

// ProofWithKey represens key-proof pair.
type ProofWithKey struct {
	Key   []byte
	Proof [][]byte
}

// GetProof is a result of getproof RPC.
type GetProof struct {
	Result  ProofWithKey `json:"proof"`
	Success bool         `json:"success"`
}

// VerifyProof is a result of verifyproof RPC.
// nil Value is considered invalid.
type VerifyProof struct {
	Value []byte
}

// MarshalJSON implements json.Marshaler.
func (p *ProofWithKey) MarshalJSON() ([]byte, error) {
	w := io.NewBufBinWriter()
	p.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return nil, w.Err
	}
	return []byte(`"` + hex.EncodeToString(w.Bytes()) + `"`), nil
}

// EncodeBinary implements io.Serializable.
func (p *ProofWithKey) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(p.Key)
	w.WriteVarUint(uint64(len(p.Proof)))
	for i := range p.Proof {
		w.WriteVarBytes(p.Proof[i])
	}
}

// DecodeBinary implements io.Serializable.
func (p *ProofWithKey) DecodeBinary(r *io.BinReader) {
	p.Key = r.ReadVarBytes()
	sz := r.ReadVarUint()
	for i := uint64(0); i < sz; i++ {
		p.Proof = append(p.Proof, r.ReadVarBytes())
	}
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *ProofWithKey) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return p.FromString(s)
}

// String implements fmt.Stringer.
func (p *ProofWithKey) String() string {
	w := io.NewBufBinWriter()
	p.EncodeBinary(w.BinWriter)
	return hex.EncodeToString(w.Bytes())
}

// FromString decodes p from hex-encoded string.
func (p *ProofWithKey) FromString(s string) error {
	rawProof, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	r := io.NewBinReaderFromBuf(rawProof)
	p.DecodeBinary(r)
	return r.Err
}

// MarshalJSON implements json.Marshaler.
func (p *VerifyProof) MarshalJSON() ([]byte, error) {
	if p.Value == nil {
		return []byte(`"invalid"`), nil
	}
	return []byte(`{"value":"` + hex.EncodeToString(p.Value) + `"}`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *VerifyProof) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`"invalid"`)) {
		p.Value = nil
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if len(m) != 1 {
		return errors.New("must have single key")
	}
	v, ok := m["value"]
	if !ok {
		return errors.New("invalid json")
	}
	b, err := hex.DecodeString(v)
	if err != nil {
		return err
	}
	p.Value = b
	return nil
}
