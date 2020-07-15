package manifest

import (
	"encoding/hex"
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Parameter represents smartcontract's parameter's definition.
type Parameter struct {
	Name string                  `json:"name"`
	Type smartcontract.ParamType `json:"type"`
}

// Event is a description of a single event.
type Event struct {
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters"`
}

// Group represents a group of smartcontracts identified by a public key.
// Every SC in a group must provide signature of it's hash to prove
// it belongs to a group.
type Group struct {
	PublicKey *keys.PublicKey `json:"pubKey"`
	Signature []byte          `json:"signature"`
}

type groupAux struct {
	PublicKey string `json:"pubKey"`
	Signature []byte `json:"signature"`
}

// Method represents method's metadata.
type Method struct {
	Name       string                  `json:"name"`
	Parameters []Parameter             `json:"parameters"`
	ReturnType smartcontract.ParamType `json:"returnType"`
}

// NewParameter returns new paramter with the specified name and type.
func NewParameter(name string, typ smartcontract.ParamType) Parameter {
	return Parameter{
		Name: name,
		Type: typ,
	}
}

// DefaultEntryPoint represents default entrypoint to a contract.
func DefaultEntryPoint() *Method {
	return &Method{
		Name: "Main",
		Parameters: []Parameter{
			NewParameter("operation", smartcontract.StringType),
			NewParameter("args", smartcontract.ArrayType),
		},
		ReturnType: smartcontract.AnyType,
	}
}

// IsValid checks whether group's signature corresponds to the given hash.
func (g *Group) IsValid(h util.Uint160) bool {
	return g.PublicKey.Verify(g.Signature, hash.Sha256(h.BytesBE()).BytesBE())
}

// MarshalJSON implements json.Marshaler interface.
func (g *Group) MarshalJSON() ([]byte, error) {
	aux := &groupAux{
		PublicKey: hex.EncodeToString(g.PublicKey.Bytes()),
		Signature: g.Signature,
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (g *Group) UnmarshalJSON(data []byte) error {
	aux := new(groupAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	b, err := hex.DecodeString(aux.PublicKey)
	if err != nil {
		return err
	}
	pub := new(keys.PublicKey)
	if err := pub.DecodeBytes(b); err != nil {
		return err
	}
	g.PublicKey = pub
	g.Signature = aux.Signature
	return nil
}
