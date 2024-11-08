package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	ojson "github.com/nspcc-dev/go-ordered-json"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxManifestSize is the max length for a valid contract manifest.
	MaxManifestSize = math.MaxUint16

	// NEP11StandardName represents the name of NEP-11 smartcontract standard.
	NEP11StandardName = "NEP-11"
	// NEP17StandardName represents the name of NEP-17 smartcontract standard.
	NEP17StandardName = "NEP-17"
	// NEP11Payable represents the name of contract interface which can receive NEP-11 tokens.
	NEP11Payable = "NEP-11-Payable"
	// NEP17Payable represents the name of contract interface which can receive NEP-17 tokens.
	NEP17Payable = "NEP-17-Payable"
	// NEP24StandardName represents the name of the NEP-24 smart contract standard for NFT royalties.
	NEP24StandardName = "NEP-24"
	// NEP24Payable represents the name of the contract interface for handling royalty payments in accordance
	// with the NEP-24 standard.
	NEP24Payable = "NEP-24-Payable"

	emptyFeatures = "{}"
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

// NewManifest returns a new manifest with necessary fields initialized.
func NewManifest(name string) *Manifest {
	m := &Manifest{
		Name: name,
		ABI: ABI{
			Methods: []Method{},
			Events:  []Event{},
		},
		Features:           json.RawMessage(emptyFeatures),
		Groups:             []Group{},
		Permissions:        []Permission{},
		SupportedStandards: []string{},
		Extra:              json.RawMessage("null"),
	}
	m.Trusts.Restrict()
	return m
}

// DefaultManifest returns the default contract manifest.
func DefaultManifest(name string) *Manifest {
	m := NewManifest(name)
	m.Permissions = []Permission{*NewPermission(PermissionWildcard)}
	return m
}

// CanCall returns true if the current contract is allowed to call
// the method of another contract with the specified hash.
func (m *Manifest) CanCall(hash util.Uint160, toCall *Manifest, method string) bool {
	return slices.ContainsFunc(m.Permissions, func(p Permission) bool {
		return p.IsAllowed(hash, toCall, method)
	})
}

// IsValid checks manifest internal consistency and correctness, one of the
// checks is for group signature correctness, contract hash is passed for it.
// If hash is empty, then hash-related checks are omitted.
func (m *Manifest) IsValid(hash util.Uint160, checkSize bool) error {
	var err error

	if m.Name == "" {
		return errors.New("no name")
	}

	if slices.Contains(m.SupportedStandards, "") {
		return errors.New("invalid nameless supported standard")
	}
	if sliceHasDups(m.SupportedStandards, strings.Compare) {
		return errors.New("duplicate supported standards")
	}
	err = m.ABI.IsValid()
	if err != nil {
		return fmt.Errorf("ABI: %w", err)
	}

	if strings.Map(func(c rune) rune {
		switch c {
		case ' ', '\n', '\t', '\r': // Strip all JSON whitespace.
			return -1
		}
		return c
	}, string(m.Features)) != emptyFeatures { // empty struct should be left.
		return errors.New("invalid features")
	}
	err = Groups(m.Groups).AreValid(hash)
	if err != nil {
		return err
	}
	if m.Trusts.Value == nil && !m.Trusts.Wildcard {
		return errors.New("invalid (null?) trusts")
	}
	if sliceHasDups(m.Trusts.Value, PermissionDesc.Compare) {
		return errors.New("duplicate trusted contracts")
	}
	err = Permissions(m.Permissions).AreValid()
	if err != nil {
		return err
	}
	if !checkSize {
		return nil
	}
	si, err := m.ToStackItem()
	if err != nil {
		return fmt.Errorf("failed to check manifest serialisation: %w", err)
	}
	_, err = stackitem.Serialize(si)
	if err != nil {
		return fmt.Errorf("manifest is not serializable: %w", err)
	}
	return nil
}

// IsStandardSupported denotes whether the specified standard is supported by the contract.
func (m *Manifest) IsStandardSupported(standard string) bool {
	return slices.Contains(m.SupportedStandards, standard)
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
	extra := extraToStackItem(m.Extra)
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

// extraToStackItem removes indentation from `Extra` field in JSON and
// converts it to a byte-array stack item.
func extraToStackItem(rawExtra []byte) stackitem.Item {
	extra := stackitem.Make("null")
	if rawExtra == nil || string(rawExtra) == "null" {
		return extra
	}

	d := ojson.NewDecoder(bytes.NewReader(rawExtra))
	// The result is put directly in the database and affects state-root calculation,
	// thus use ordered map to stay compatible with C# implementation.
	d.UseOrderedObject()
	// Prevent accidental precision loss.
	d.UseNumber()

	var obj any

	// The error can't really occur because `json.RawMessage` is already a valid json.
	_ = d.Decode(&obj)
	res, _ := ojson.Marshal(obj)
	return stackitem.NewByteArray(res)
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
	m.Features = json.RawMessage(emptyFeatures)
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
		m.Trusts = WildPermissionDescs{Value: nil, Wildcard: true} // wildcard by default
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
