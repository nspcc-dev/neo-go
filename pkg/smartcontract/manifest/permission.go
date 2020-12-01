package manifest

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
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
