package result

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
)

// Validator is used for the representation of consensus node data in the JSON-RPC
// protocol.
type Validator struct {
	PublicKey keys.PublicKey `json:"publickey"`
	Votes     int64          `json:"votes"`
}

// Candidate represents a node participating in the governance elections, it's
// active when it's a validator (consensus node).
type Candidate struct {
	PublicKey keys.PublicKey `json:"publickey"`
	Votes     int64          `json:"votes,string"`
	Active    bool           `json:"active"`
}

type newValidator struct {
	PublicKey keys.PublicKey `json:"publickey"`
	Votes     int64          `json:"votes"`
}

type oldValidator struct {
	PublicKey keys.PublicKey `json:"publickey"`
	Votes     int64          `json:"votes,string"`
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (v *Validator) UnmarshalJSON(data []byte) error {
	var nv = new(newValidator)
	err := json.Unmarshal(data, nv)
	if err != nil {
		var ov = new(oldValidator)
		err := json.Unmarshal(data, ov)
		if err != nil {
			return err
		}
		v.PublicKey = ov.PublicKey
		v.Votes = ov.Votes
		return nil
	}
	v.PublicKey = nv.PublicKey
	v.Votes = nv.Votes
	return nil
}
