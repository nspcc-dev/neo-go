package manifest

import (
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
	PublicKey *keys.PublicKey `json:"pubkey"`
	Signature []byte          `json:"signature"`
}

type groupAux struct {
	PublicKey string `json:"pubkey"`
	Signature []byte `json:"signature"`
}

// Method represents method's metadata.
type Method struct {
	Name       string                  `json:"name"`
	Offset     int                     `json:"offset"`
	Parameters []Parameter             `json:"parameters"`
	ReturnType smartcontract.ParamType `json:"returntype"`
	Safe       bool                    `json:"safe"`
}

// NewParameter returns new parameter of specified name and type.
func NewParameter(name string, typ smartcontract.ParamType) Parameter {
	return Parameter{
		Name: name,
		Type: typ,
	}
}

// IsValid checks whether group's signature corresponds to the given hash.
func (g *Group) IsValid(h util.Uint160) error {
	if !g.PublicKey.Verify(g.Signature, hash.Sha256(h.BytesBE()).BytesBE()) {
		return errors.New("incorrect group signature")
	}
	return nil
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

// ToStackItem converts Group to stackitem.Item.
func (g *Group) ToStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray(g.PublicKey.Bytes()),
		stackitem.NewByteArray(g.Signature),
	})
}

// FromStackItem converts stackitem.Item to Group.
func (g *Group) FromStackItem(item stackitem.Item) error {
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Group stackitem type")
	}
	group := item.Value().([]stackitem.Item)
	if len(group) != 2 {
		return errors.New("invalid Group stackitem length")
	}
	pKey, err := group[0].TryBytes()
	if err != nil {
		return err
	}
	g.PublicKey, err = keys.NewPublicKeyFromBytes(pKey, elliptic.P256())
	if err != nil {
		return err
	}
	sig, err := group[1].TryBytes()
	if err != nil {
		return err
	}
	g.Signature = sig
	return nil
}

// ToStackItem converts Method to stackitem.Item.
func (m *Method) ToStackItem() stackitem.Item {
	params := make([]stackitem.Item, len(m.Parameters))
	for i := range m.Parameters {
		params[i] = m.Parameters[i].ToStackItem()
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(m.Name),
		stackitem.Make(params),
		stackitem.Make(int(m.ReturnType)),
		stackitem.Make(m.Offset),
		stackitem.Make(m.Safe),
	})
}

// FromStackItem converts stackitem.Item to Method.
func (m *Method) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Method stackitem type")
	}
	method := item.Value().([]stackitem.Item)
	if len(method) != 5 {
		return errors.New("invalid Method stackitem length")
	}
	m.Name, err = stackitem.ToString(method[0])
	if err != nil {
		return err
	}
	if method[1].Type() != stackitem.ArrayT {
		return errors.New("invalid Params stackitem type")
	}
	params := method[1].Value().([]stackitem.Item)
	m.Parameters = make([]Parameter, len(params))
	for i := range params {
		p := new(Parameter)
		if err := p.FromStackItem(params[i]); err != nil {
			return err
		}
		m.Parameters[i] = *p
	}
	rTyp, err := method[2].TryInteger()
	if err != nil {
		return err
	}
	m.ReturnType, err = smartcontract.ConvertToParamType(int(rTyp.Int64()))
	if err != nil {
		return err
	}
	offset, err := method[3].TryInteger()
	if err != nil {
		return err
	}
	m.Offset = int(offset.Int64())
	safe, err := method[4].TryBool()
	if err != nil {
		return err
	}
	m.Safe = safe
	return nil
}

// ToStackItem converts Parameter to stackitem.Item.
func (p *Parameter) ToStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(p.Name),
		stackitem.Make(int(p.Type)),
	})
}

// FromStackItem converts stackitem.Item to Parameter.
func (p *Parameter) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Parameter stackitem type")
	}
	param := item.Value().([]stackitem.Item)
	if len(param) != 2 {
		return errors.New("invalid Parameter stackitem length")
	}
	p.Name, err = stackitem.ToString(param[0])
	if err != nil {
		return err
	}
	typ, err := param[1].TryInteger()
	if err != nil {
		return err
	}
	p.Type, err = smartcontract.ConvertToParamType(int(typ.Int64()))
	if err != nil {
		return err
	}
	return nil
}

// ToStackItem converts Event to stackitem.Item.
func (e *Event) ToStackItem() stackitem.Item {
	params := make([]stackitem.Item, len(e.Parameters))
	for i := range e.Parameters {
		params[i] = e.Parameters[i].ToStackItem()
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(e.Name),
		stackitem.Make(params),
	})
}

// FromStackItem converts stackitem.Item to Event.
func (e *Event) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Event stackitem type")
	}
	event := item.Value().([]stackitem.Item)
	if len(event) != 2 {
		return errors.New("invalid Event stackitem length")
	}
	e.Name, err = stackitem.ToString(event[0])
	if err != nil {
		return err
	}
	if event[1].Type() != stackitem.ArrayT {
		return errors.New("invalid Params stackitem type")
	}
	params := event[1].Value().([]stackitem.Item)
	e.Parameters = make([]Parameter, len(params))
	for i := range params {
		p := new(Parameter)
		if err := p.FromStackItem(params[i]); err != nil {
			return err
		}
		e.Parameters[i] = *p
	}
	return nil
}
