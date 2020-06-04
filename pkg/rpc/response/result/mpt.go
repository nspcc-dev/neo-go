package result

import (
	"encoding/hex"
	"encoding/json"

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
