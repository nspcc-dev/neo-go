package manifest

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxManifestSize is a max length for a valid contract manifest.
	MaxManifestSize = math.MaxUint16

	// MethodInit is a name for default initialization method.
	MethodInit = "_initialize"

	// MethodDeploy is a name for default method called during contract deployment.
	MethodDeploy = "_deploy"

	// MethodVerify is a name for default verification method.
	MethodVerify = "verify"

	// MethodOnNEP17Payment is name of the method which is called when contract receives NEP-17 tokens.
	MethodOnNEP17Payment = "onNEP17Payment"

	// MethodOnNEP11Payment is the name of the method which is called when contract receives NEP-11 tokens.
	MethodOnNEP11Payment = "onNEP11Payment"

	// NEP10StandardName represents the name of NEP10 smartcontract standard.
	NEP10StandardName = "NEP-10"
	// NEP17StandardName represents the name of NEP17 smartcontract standard.
	NEP17StandardName = "NEP-17"
)

// ABI represents a contract application binary interface.
type ABI struct {
	Methods []Method `json:"methods"`
	Events  []Event  `json:"events"`
}

// Manifest represens contract metadata.
type Manifest struct {
	// Name is a contract's name.
	Name string `json:"name"`
	// ABI is a contract's ABI.
	ABI ABI `json:"abi"`
	// Groups is a set of groups to which a contract belongs.
	Groups      []Group      `json:"groups"`
	Permissions []Permission `json:"permissions"`
	// SupportedStandards is a list of standards supported by the contract.
	SupportedStandards []string `json:"supportedstandards"`
	// Trusts is a set of hashes to a which contract trusts.
	Trusts WildUint160s `json:"trusts"`
	// Extra is an implementation-defined user data.
	Extra interface{} `json:"extra"`
}

// NewManifest returns new manifest with necessary fields initialized.
func NewManifest(name string) *Manifest {
	m := &Manifest{
		Name: name,
		ABI: ABI{
			Methods: []Method{},
			Events:  []Event{},
		},
		Groups:             []Group{},
		Permissions:        []Permission{},
		SupportedStandards: []string{},
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

// GetMethod returns methods with the specified name.
func (a *ABI) GetMethod(name string, paramCount int) *Method {
	for i := range a.Methods {
		if a.Methods[i].Name == name && (paramCount == -1 || len(a.Methods[i].Parameters) == paramCount) {
			return &a.Methods[i]
		}
	}
	return nil
}

// GetEvent returns event with the specified name.
func (a *ABI) GetEvent(name string) *Event {
	for i := range a.Events {
		if a.Events[i].Name == name {
			return &a.Events[i]
		}
	}
	return nil
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

// IsValid checks whether the hash given is correct wrt manifest's groups.
func (m *Manifest) IsValid(hash util.Uint160) bool {
	for _, g := range m.Groups {
		if !g.IsValid(hash) {
			return false
		}
	}
	return true
}

// EncodeBinary implements io.Serializable.
func (m *Manifest) EncodeBinary(w *io.BinWriter) {
	data, err := json.Marshal(m)
	if err != nil {
		w.Err = err
		return
	}
	w.WriteVarBytes(data)
}

// DecodeBinary implements io.Serializable.
func (m *Manifest) DecodeBinary(r *io.BinReader) {
	data := r.ReadVarBytes(MaxManifestSize)
	if r.Err != nil {
		return
	} else if err := json.Unmarshal(data, m); err != nil {
		r.Err = err
	}
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
		for i := range m.Trusts.Value {
			tItems[i] = stackitem.NewByteArray(m.Trusts.Value[i].BytesBE())
		}
		trusts = stackitem.Make(tItems)
	}
	extra := stackitem.Make("null")
	if m.Extra != nil {
		e, err := json.Marshal(m.Extra)
		if err != nil {
			return nil, err
		}
		extra = stackitem.NewByteArray(e)
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(m.Name),
		stackitem.Make(groups),
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
	if len(str) != 7 {
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
	if str[2].Type() != stackitem.ArrayT {
		return errors.New("invalid SupportedStandards stackitem type")
	}
	supportedStandards := str[2].Value().([]stackitem.Item)
	m.SupportedStandards = make([]string, len(supportedStandards))
	for i := range supportedStandards {
		m.SupportedStandards[i], err = stackitem.ToString(supportedStandards[i])
		if err != nil {
			return err
		}
	}
	abi := new(ABI)
	if err := abi.FromStackItem(str[3]); err != nil {
		return err
	}
	m.ABI = *abi
	if str[4].Type() != stackitem.ArrayT {
		return errors.New("invalid Permissions stackitem type")
	}
	permissions := str[4].Value().([]stackitem.Item)
	m.Permissions = make([]Permission, len(permissions))
	for i := range permissions {
		p := new(Permission)
		if err := p.FromStackItem(permissions[i]); err != nil {
			return err
		}
		m.Permissions[i] = *p
	}
	if _, ok := str[5].(stackitem.Null); ok {
		m.Trusts.Restrict()
	} else {
		if str[5].Type() != stackitem.ArrayT {
			return errors.New("invalid Trusts stackitem type")
		}
		trusts := str[5].Value().([]stackitem.Item)
		m.Trusts = WildUint160s{Value: make([]util.Uint160, len(trusts))}
		for i := range trusts {
			bytes, err := trusts[i].TryBytes()
			if err != nil {
				return err
			}
			m.Trusts.Value[i], err = util.Uint160DecodeBytesBE(bytes)
			if err != nil {
				return err
			}
		}
	}
	extra, err := str[6].TryBytes()
	if err != nil {
		return err
	}
	if string(extra) == "null" {
		return nil
	}
	return json.Unmarshal(extra, &m.Extra)
}

// ToStackItem converts ABI to stackitem.Item.
func (a *ABI) ToStackItem() stackitem.Item {
	methods := make([]stackitem.Item, len(a.Methods))
	for i := range a.Methods {
		methods[i] = a.Methods[i].ToStackItem()
	}
	events := make([]stackitem.Item, len(a.Events))
	for i := range a.Events {
		events[i] = a.Events[i].ToStackItem()
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(methods),
		stackitem.Make(events),
	})
}

// FromStackItem converts stackitem.Item to ABI.
func (a *ABI) FromStackItem(item stackitem.Item) error {
	if item.Type() != stackitem.StructT {
		return errors.New("invalid ABI stackitem type")
	}
	str := item.Value().([]stackitem.Item)
	if len(str) != 2 {
		return errors.New("invalid ABI stackitem length")
	}
	if str[0].Type() != stackitem.ArrayT {
		return errors.New("invalid Methods stackitem type")
	}
	methods := str[0].Value().([]stackitem.Item)
	a.Methods = make([]Method, len(methods))
	for i := range methods {
		m := new(Method)
		if err := m.FromStackItem(methods[i]); err != nil {
			return err
		}
		a.Methods[i] = *m
	}
	if str[1].Type() != stackitem.ArrayT {
		return errors.New("invalid Events stackitem type")
	}
	events := str[1].Value().([]stackitem.Item)
	a.Events = make([]Event, len(events))
	for i := range events {
		e := new(Event)
		if err := e.FromStackItem(events[i]); err != nil {
			return err
		}
		a.Events[i] = *e
	}
	return nil
}
