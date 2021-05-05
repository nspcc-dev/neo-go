package manifest

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxManifestSize is a max length for a valid contract manifest.
	MaxManifestSize = math.MaxUint16

	// NEP11StandardName represents the name of NEP11 smartcontract standard.
	NEP11StandardName = "NEP-11"
	// NEP17StandardName represents the name of NEP17 smartcontract standard.
	NEP17StandardName = "NEP-17"
	// NEP11Payable represents the name of contract interface which can receive NEP-11 tokens.
	NEP11Payable = "NEP-11-Payable"
	// NEP17Payable represents the name of contract interface which can receive NEP-17 tokens.
	NEP17Payable = "NEP-17-Payable"
)

// Manifest represens contract metadata.
type Manifest struct {
	// Name is a contract's name.
	Name string `json:"name"`
	// ABI is a contract's ABI.
	ABI ABI `json:"abi"`
	// Features is a set of contract features. Currently unused.
	Features json.RawMessage `json:"features"`
	// Groups is a set of groups to which a contract belongs.
	Groups      []Group      `json:"groups"`
	Permissions []Permission `json:"permissions"`
	// SupportedStandards is a list of standards supported by the contract.
	SupportedStandards []string `json:"supportedstandards"`
	// Trusts is a set of hashes to a which contract trusts.
	Trusts WildPermissionDescs `json:"trusts"`
	// Extra is an implementation-defined user data.
	Extra json.RawMessage `json:"extra"`
}

// NewManifest returns new manifest with necessary fields initialized.
func NewManifest(name string) *Manifest {
	m := &Manifest{
		Name: name,
		ABI: ABI{
			Methods: []Method{},
			Events:  []Event{},
		},
		Features:           json.RawMessage("{}"),
		Groups:             []Group{},
		Permissions:        []Permission{},
		SupportedStandards: []string{},
		Extra:              json.RawMessage("null"),
	}
	m.Trusts.Restrict()
	return m
}

// DefaultManifest returns default contract manifest.
func DefaultManifest(name string) *Manifest {
	m := NewManifest(name)
	m.Permissions = []Permission{*NewPermission(PermissionWildcard)}
	return m
}

// CanCall returns true is current contract is allowed to call
// method of another contract with specified hash.
func (m *Manifest) CanCall(hash util.Uint160, toCall *Manifest, method string) bool {
	for i := range m.Permissions {
		if m.Permissions[i].IsAllowed(hash, toCall, method) {
			return true
		}
	}
	return false
}

// IsValid checks manifest internal consistency and correctness, one of the
// checks is for group signature correctness, contract hash is passed for it.
func (m *Manifest) IsValid(hash util.Uint160) error {
	var err error

	if m.Name == "" {
		return errors.New("no name")
	}

	for i := range m.SupportedStandards {
		if m.SupportedStandards[i] == "" {
			return errors.New("invalid nameless supported standard")
		}
	}
	if len(m.SupportedStandards) > 1 {
		names := make([]string, len(m.SupportedStandards))
		copy(names, m.SupportedStandards)
		if stringsHaveDups(names) {
			return errors.New("duplicate supported standards")
		}
	}
	err = m.ABI.IsValid()
	if err != nil {
		return err
	}
	err = Groups(m.Groups).AreValid(hash)
	if err != nil {
		return err
	}
	if len(m.Trusts.Value) > 1 {
		hashes := make([]PermissionDesc, len(m.Trusts.Value))
		copy(hashes, m.Trusts.Value)
		if permissionDescsHaveDups(hashes) {
			return errors.New("duplicate trusted contracts")
		}
	}
	return Permissions(m.Permissions).AreValid()
}

// ToStackItem converts Manifest to stackitem.Item.
func (m *Manifest) ToStackItem() (stackitem.Item, error) {
	groups := make([]stackitem.Item, len(m.Groups))
	for i := range m.Groups {
		groups[i] = m.Groups[i].ToStackItem()
	}
	supportedStandards := make([]stackitem.Item, len(m.SupportedStandards))
	for i := range m.SupportedStandards {
		supportedStandards[i] = stackitem.Make(m.SupportedStandards[i])
	}
	abi := m.ABI.ToStackItem()
	permissions := make([]stackitem.Item, len(m.Permissions))
	for i := range m.Permissions {
		permissions[i] = m.Permissions[i].ToStackItem()
	}
	trusts := stackitem.Item(stackitem.Null{})
	if !m.Trusts.IsWildcard() {
		tItems := make([]stackitem.Item, len(m.Trusts.Value))
		for i, v := range m.Trusts.Value {
			tItems[i] = v.ToStackItem()
		}
		trusts = stackitem.Make(tItems)
	}
	extra := stackitem.Make("null")
	if m.Extra != nil {
		extra = stackitem.NewByteArray(m.Extra)
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(m.Name),
		stackitem.Make(groups),
		stackitem.NewMap(),
		stackitem.Make(supportedStandards),
		abi,
		stackitem.Make(permissions),
		trusts,
		extra,
	}), nil
}

// FromStackItem converts stackitem.Item to Manifest.
func (m *Manifest) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Manifest stackitem type")
	}
	str := item.Value().([]stackitem.Item)
	if len(str) != 8 {
		return errors.New("invalid stackitem length")
	}
	m.Name, err = stackitem.ToString(str[0])
	if err != nil {
		return err
	}
	if str[1].Type() != stackitem.ArrayT {
		return errors.New("invalid Groups stackitem type")
	}
	groups := str[1].Value().([]stackitem.Item)
	m.Groups = make([]Group, len(groups))
	for i := range groups {
		group := new(Group)
		err := group.FromStackItem(groups[i])
		if err != nil {
			return err
		}
		m.Groups[i] = *group
	}
	if str[2].Type() != stackitem.MapT || str[2].(*stackitem.Map).Len() != 0 {
		return errors.New("invalid Features stackitem")
	}
	m.Features = json.RawMessage("{}")
	if str[3].Type() != stackitem.ArrayT {
		return errors.New("invalid SupportedStandards stackitem type")
	}
	supportedStandards := str[3].Value().([]stackitem.Item)
	m.SupportedStandards = make([]string, len(supportedStandards))
	for i := range supportedStandards {
		m.SupportedStandards[i], err = stackitem.ToString(supportedStandards[i])
		if err != nil {
			return err
		}
	}
	abi := new(ABI)
	if err := abi.FromStackItem(str[4]); err != nil {
		return err
	}
	m.ABI = *abi
	if str[5].Type() != stackitem.ArrayT {
		return errors.New("invalid Permissions stackitem type")
	}
	permissions := str[5].Value().([]stackitem.Item)
	m.Permissions = make([]Permission, len(permissions))
	for i := range permissions {
		p := new(Permission)
		if err := p.FromStackItem(permissions[i]); err != nil {
			return err
		}
		m.Permissions[i] = *p
	}
	if _, ok := str[6].(stackitem.Null); ok {
		m.Trusts.Restrict()
	} else {
		if str[6].Type() != stackitem.ArrayT {
			return errors.New("invalid Trusts stackitem type")
		}
		trusts := str[6].Value().([]stackitem.Item)
		m.Trusts = WildPermissionDescs{Value: make([]PermissionDesc, len(trusts))}
		for i := range trusts {
			v := new(PermissionDesc)
			err = v.FromStackItem(trusts[i])
			if err != nil {
				return err
			}
			m.Trusts.Value[i] = *v
		}
	}
	extra, err := str[7].TryBytes()
	if err != nil {
		return err
	}
	m.Extra = extra
	return nil
}
