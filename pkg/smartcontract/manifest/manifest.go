package manifest

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// MaxManifestSize is a max length for a valid contract manifest.
	MaxManifestSize = 4096

	// MethodInit is a name for default initialization method.
	MethodInit = "_initialize"

	// MethodVerify is a name for default verification method.
	MethodVerify = "verify"

	// NEP5StandardName represents the name of NEP5 smartcontract standard.
	NEP5StandardName = "NEP-5"
	// NEP10StandardName represents the name of NEP10 smartcontract standard.
	NEP10StandardName = "NEP-10"
)

// ABI represents a contract application binary interface.
type ABI struct {
	Hash    util.Uint160 `json:"hash"`
	Methods []Method     `json:"methods"`
	Events  []Event      `json:"events"`
}

// Manifest represens contract metadata.
type Manifest struct {
	// ABI is a contract's ABI.
	ABI ABI
	// Groups is a set of groups to which a contract belongs.
	Groups []Group
	// Features is a set of contract's features.
	Features    smartcontract.PropertyState
	Permissions []Permission
	// SupportedStandards is a list of standards supported by the contract.
	SupportedStandards []string
	// Trusts is a set of hashes to a which contract trusts.
	Trusts WildUint160s
	// SafeMethods is a set of names of safe methods.
	SafeMethods WildStrings
	// Extra is an implementation-defined user data.
	Extra interface{}
}

type manifestAux struct {
	ABI                *ABI            `json:"abi"`
	Groups             []Group         `json:"groups"`
	Features           map[string]bool `json:"features"`
	Permissions        []Permission    `json:"permissions"`
	SupportedStandards []string        `json:"supportedstandards"`
	Trusts             *WildUint160s   `json:"trusts"`
	SafeMethods        *WildStrings    `json:"safemethods"`
	Extra              interface{}     `json:"extra"`
}

// NewManifest returns new manifest with necessary fields initialized.
func NewManifest(h util.Uint160) *Manifest {
	m := &Manifest{
		ABI: ABI{
			Hash:    h,
			Methods: []Method{},
			Events:  []Event{},
		},
		Groups:             []Group{},
		Features:           smartcontract.NoProperties,
		SupportedStandards: []string{},
	}
	m.Trusts.Restrict()
	m.SafeMethods.Restrict()
	return m
}

// DefaultManifest returns default contract manifest.
func DefaultManifest(h util.Uint160) *Manifest {
	m := NewManifest(h)
	m.Permissions = []Permission{*NewPermission(PermissionWildcard)}
	return m
}

// GetMethod returns methods with the specified name.
func (a *ABI) GetMethod(name string) *Method {
	for i := range a.Methods {
		if a.Methods[i].Name == name {
			return &a.Methods[i]
		}
	}
	return nil
}

// CanCall returns true is current contract is allowed to call
// method of another contract.
func (m *Manifest) CanCall(toCall *Manifest, method string) bool {
	// this if is not present in the original code but should probably be here
	if toCall.SafeMethods.Contains(method) {
		return true
	}
	for i := range m.Permissions {
		if m.Permissions[i].IsAllowed(toCall, method) {
			return true
		}
	}
	return false
}

// IsValid checks whether the given hash is the one specified in manifest and
// verifies it against all the keys in manifest groups.
func (m *Manifest) IsValid(hash util.Uint160) bool {
	if m.ABI.Hash != hash {
		return false
	}
	for _, g := range m.Groups {
		if !g.IsValid(hash) {
			return false
		}
	}
	return true
}

// MarshalJSON implements json.Marshaler interface.
func (m *Manifest) MarshalJSON() ([]byte, error) {
	features := make(map[string]bool)
	features["storage"] = m.Features&smartcontract.HasStorage != 0
	features["payable"] = m.Features&smartcontract.IsPayable != 0
	aux := &manifestAux{
		ABI:                &m.ABI,
		Groups:             m.Groups,
		Features:           features,
		Permissions:        m.Permissions,
		SupportedStandards: m.SupportedStandards,
		Trusts:             &m.Trusts,
		SafeMethods:        &m.SafeMethods,
		Extra:              m.Extra,
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (m *Manifest) UnmarshalJSON(data []byte) error {
	aux := &manifestAux{
		ABI:         &m.ABI,
		Trusts:      &m.Trusts,
		SafeMethods: &m.SafeMethods,
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if aux.Features["storage"] {
		m.Features |= smartcontract.HasStorage
	}
	if aux.Features["payable"] {
		m.Features |= smartcontract.IsPayable
	}

	m.Groups = aux.Groups
	m.Permissions = aux.Permissions
	m.SupportedStandards = aux.SupportedStandards
	m.Extra = aux.Extra

	return nil
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
	data := r.ReadVarBytes()
	if r.Err != nil {
		return
	} else if err := json.Unmarshal(data, m); err != nil {
		r.Err = err
	}
}
