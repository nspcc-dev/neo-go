package manifest

import (
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// PermissionType represents permission type.
type PermissionType uint8

const (
	// PermissionWildcard allows everything.
	PermissionWildcard PermissionType = 0
	// PermissionHash restricts called contracts based on hash.
	PermissionHash PermissionType = 1
	// PermissionGroup restricts called contracts based on public key.
	PermissionGroup PermissionType = 2
)

// PermissionDesc is a permission descriptor.
type PermissionDesc struct {
	Type  PermissionType
	Value interface{}
}

// Permission describes which contracts may be invoked and which methods are called.
type Permission struct {
	Contract PermissionDesc `json:"contract"`
	Methods  WildStrings    `json:"methods"`
}

// Permissions is just an array of Permission.
type Permissions []Permission

type permissionAux struct {
	Contract PermissionDesc `json:"contract"`
	Methods  WildStrings    `json:"methods"`
}

// NewPermission returns new permission of a given type.
func NewPermission(typ PermissionType, args ...interface{}) *Permission {
	return &Permission{
		Contract: *newPermissionDesc(typ, args...),
	}
}

func newPermissionDesc(typ PermissionType, args ...interface{}) *PermissionDesc {
	desc := &PermissionDesc{Type: typ}
	switch typ {
	case PermissionWildcard:
		if len(args) != 0 {
			panic("wildcard permission has no arguments")
		}
	case PermissionHash:
		if len(args) == 0 {
			panic("hash permission should have an argument")
		} else if u, ok := args[0].(util.Uint160); !ok {
			panic("hash permission should have util.Uint160 argument")
		} else {
			desc.Value = u
		}
	case PermissionGroup:
		if len(args) == 0 {
			panic("group permission should have an argument")
		} else if pub, ok := args[0].(*keys.PublicKey); !ok {
			panic("group permission should have a public key argument")
		} else {
			desc.Value = pub
		}
	}
	return desc
}

// Hash returns hash for hash-permission.
func (d *PermissionDesc) Hash() util.Uint160 {
	return d.Value.(util.Uint160)
}

// Group returns group's public key for group-permission.
func (d *PermissionDesc) Group() *keys.PublicKey {
	return d.Value.(*keys.PublicKey)
}

// Less returns true if this value is less than given PermissionDesc value.
func (d *PermissionDesc) Less(d1 PermissionDesc) bool {
	if d.Type < d1.Type {
		return true
	}
	if d.Type != d1.Type {
		return false
	}
	switch d.Type {
	case PermissionHash:
		return d.Hash().Less(d1.Hash())
	case PermissionGroup:
		return d.Group().Cmp(d1.Group()) < 0
	}
	return false
}

// Equals returns true if both PermissionDesc values are the same.
func (d *PermissionDesc) Equals(v PermissionDesc) bool {
	if d.Type != v.Type {
		return false
	}
	switch d.Type {
	case PermissionHash:
		return d.Hash().Equals(v.Hash())
	case PermissionGroup:
		return d.Group().Cmp(v.Group()) == 0
	}
	return false
}

// IsValid checks if Permission is correct.
func (p *Permission) IsValid() error {
	for i := range p.Methods.Value {
		if p.Methods.Value[i] == "" {
			return errors.New("empty method name")
		}
	}
	if len(p.Methods.Value) < 2 {
		return nil
	}
	names := make([]string, len(p.Methods.Value))
	copy(names, p.Methods.Value)
	if stringsHaveDups(names) {
		return errors.New("duplicate method names")
	}
	return nil
}

// AreValid checks each Permission and ensures there are no duplicates.
func (ps Permissions) AreValid() error {
	for i := range ps {
		err := ps[i].IsValid()
		if err != nil {
			return err
		}
	}
	if len(ps) < 2 {
		return nil
	}
	contracts := make([]PermissionDesc, 0, len(ps))
	for i := range ps {
		contracts = append(contracts, ps[i].Contract)
	}
	if permissionDescsHaveDups(contracts) {
		return errors.New("contracts have duplicates")
	}
	return nil
}

// IsAllowed checks if method is allowed to be executed.
func (p *Permission) IsAllowed(hash util.Uint160, m *Manifest, method string) bool {
	switch p.Contract.Type {
	case PermissionWildcard:
		return true
	case PermissionHash:
		if !p.Contract.Hash().Equals(hash) {
			return false
		}
	case PermissionGroup:
		g := p.Contract.Group()
		for i := range m.Groups {
			if !g.Equal(m.Groups[i].PublicKey) {
				return false
			}
		}
	default:
		panic(fmt.Sprintf("unexpected permission: %d", p.Contract.Type))
	}
	if p.Methods.IsWildcard() {
		return true
	}
	return p.Methods.Contains(method)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (p *Permission) UnmarshalJSON(data []byte) error {
	aux := new(permissionAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	p.Contract = aux.Contract
	p.Methods = aux.Methods
	return nil
}

// MarshalJSON implements json.Marshaler interface.
func (d *PermissionDesc) MarshalJSON() ([]byte, error) {
	switch d.Type {
	case PermissionHash:
		return json.Marshal("0x" + d.Hash().StringLE())
	case PermissionGroup:
		return json.Marshal(hex.EncodeToString(d.Group().Bytes()))
	default:
		return []byte(`"*"`), nil
	}
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (d *PermissionDesc) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	const uint160HexSize = 2 * util.Uint160Size
	switch len(s) {
	case 2 + uint160HexSize:
		// allow to unmarshal both hex and 0xhex forms
		if s[0] != '0' || s[1] != 'x' {
			return errors.New("invalid uint160")
		}
		s = s[2:]
		fallthrough
	case uint160HexSize:
		u, err := util.Uint160DecodeStringLE(s)
		if err != nil {
			return err
		}
		d.Type = PermissionHash
		d.Value = u
		return nil
	case 66:
		pub, err := keys.NewPublicKeyFromString(s)
		if err != nil {
			return err
		}
		d.Type = PermissionGroup
		d.Value = pub
		return nil
	case 1:
		if s == "*" {
			d.Type = PermissionWildcard
			return nil
		}
	}
	return errors.New("unknown permission")
}

// ToStackItem converts PermissionDesc to stackitem.Item.
func (d *PermissionDesc) ToStackItem() stackitem.Item {
	switch d.Type {
	case PermissionWildcard:
		return stackitem.Null{}
	case PermissionHash:
		return stackitem.NewByteArray(d.Hash().BytesBE())
	case PermissionGroup:
		return stackitem.NewByteArray(d.Group().Bytes())
	default:
		panic("unsupported PermissionDesc type")
	}
}

// FromStackItem converts stackitem.Item to PermissionDesc.
func (d *PermissionDesc) FromStackItem(item stackitem.Item) error {
	if _, ok := item.(stackitem.Null); ok {
		d.Type = PermissionWildcard
		return nil
	}
	if item.Type() != stackitem.ByteArrayT {
		return fmt.Errorf("unsupported permission descriptor type: %s", item.Type())
	}
	byteArr, err := item.TryBytes()
	if err != nil {
		return err
	}
	switch len(byteArr) {
	case util.Uint160Size:
		hash, _ := util.Uint160DecodeBytesBE(byteArr)
		d.Type = PermissionHash
		d.Value = hash
	case 33:
		pKey, err := keys.NewPublicKeyFromBytes(byteArr, elliptic.P256())
		if err != nil {
			return err
		}
		d.Type = PermissionGroup
		d.Value = pKey
	default:
		return errors.New("invalid ByteArray length")
	}
	return nil
}

// ToStackItem converts Permission to stackitem.Item.
func (p *Permission) ToStackItem() stackitem.Item {
	var methods stackitem.Item
	contract := p.Contract.ToStackItem()
	if p.Methods.IsWildcard() {
		methods = stackitem.Null{}
	} else {
		m := make([]stackitem.Item, len(p.Methods.Value))
		for i := range p.Methods.Value {
			m[i] = stackitem.Make(p.Methods.Value[i])
		}
		methods = stackitem.Make(m)
	}
	return stackitem.NewStruct([]stackitem.Item{
		contract,
		methods,
	})
}

// FromStackItem converts stackitem.Item to Permission.
func (p *Permission) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Permission stackitem type")
	}
	str := item.Value().([]stackitem.Item)
	if len(str) != 2 {
		return errors.New("invalid Permission stackitem length")
	}
	desc := new(PermissionDesc)
	err = desc.FromStackItem(str[0])
	if err != nil {
		return fmt.Errorf("invalid Contract stackitem: %w", err)
	}
	p.Contract = *desc
	if _, ok := str[1].(stackitem.Null); ok {
		p.Methods = WildStrings{Value: nil}
	} else {
		if str[1].Type() != stackitem.ArrayT {
			return errors.New("invalid Methods stackitem type")
		}
		methods := str[1].Value().([]stackitem.Item)
		p.Methods = WildStrings{
			Value: make([]string, len(methods)),
		}
		for i := range methods {
			p.Methods.Value[i], err = stackitem.ToString(methods[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}
