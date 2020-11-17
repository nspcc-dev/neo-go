package manifest

import (
	"encoding/json"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

	// MethodOnPayment is name of the method which is called when contract receives funds.
	MethodOnPayment = "onPayment"

	// NEP10StandardName represents the name of NEP10 smartcontract standard.
	NEP10StandardName = "NEP-10"
	// NEP17StandardName represents the name of NEP17 smartcontract standard.
	NEP17StandardName = "NEP-17"
)

// ABI represents a contract application binary interface.
type ABI struct {
	Hash    util.Uint160 `json:"hash"`
	Methods []Method     `json:"methods"`
	Events  []Event      `json:"events"`
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
	// SafeMethods is a set of names of safe methods.
	SafeMethods WildStrings `json:"safemethods"`
	// Extra is an implementation-defined user data.
	Extra interface{} `json:"extra"`
}

// NewManifest returns new manifest with necessary fields initialized.
func NewManifest(h util.Uint160, name string) *Manifest {
	m := &Manifest{
		Name: name,
		ABI: ABI{
			Hash:    h,
			Methods: []Method{},
			Events:  []Event{},
		},
		Groups:             []Group{},
		SupportedStandards: []string{},
	}
	m.Trusts.Restrict()
	m.SafeMethods.Restrict()
	return m
}

// DefaultManifest returns default contract manifest.
func DefaultManifest(h util.Uint160, name string) *Manifest {
	m := NewManifest(h, name)
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
