package result

import (
	"bytes"
	"encoding/base64"
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// StateHeight is a result of getstateheight RPC.
type StateHeight struct {
	Local     uint32 `json:"localrootindex"`
	Validated uint32 `json:"validatedrootindex"`
}

// ProofWithKey represens key-proof pair.
type ProofWithKey struct {
	Key   []byte
	Proof [][]byte
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
	return []byte(`"` + base64.StdEncoding.EncodeToString(w.Bytes()) + `"`), nil
}

// EncodeBinary implements io.Serializable.
func (p *ProofWithKey) EncodeBinary(w io.BinaryWriter) {
	w.WriteVarBytes(p.Key)
	w.WriteVarUint(uint64(len(p.Proof)))
	for i := range p.Proof {
		w.WriteVarBytes(p.Proof[i])
	}
}

// DecodeBinary implements io.Serializable.
func (p *ProofWithKey) DecodeBinary(r io.BinaryReader) {
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
	return base64.StdEncoding.EncodeToString(w.Bytes())
}

// FromString decodes p from hex-encoded string.
func (p *ProofWithKey) FromString(s string) error {
	rawProof, err := base64.StdEncoding.DecodeString(s)
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
	return []byte(`"` + base64.StdEncoding.EncodeToString(p.Value) + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *VerifyProof) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`"invalid"`)) {
		p.Value = nil
		return nil
	}
	var m string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	b, err := base64.StdEncoding.DecodeString(m)
	if err != nil {
		return err
	}
	p.Value = b
	return nil
}
