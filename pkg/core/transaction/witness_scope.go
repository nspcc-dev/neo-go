package transaction

//go:generate stringer -type=WitnessScope -output=witness_scope_string.go
import (
	"encoding/json"
	"fmt"
	"strings"
)

// WitnessScope represents set of witness flags for Transaction cosigner.
type WitnessScope byte

const (
	// FeeOnly is only valid for a sender, it can't be used during the execution.
	FeeOnly WitnessScope = 0
	// CalledByEntry means that this condition must hold: EntryScriptHash == CallingScriptHash.
	// No params is needed, as the witness/permission/signature given on first invocation will
	// automatically expire if entering deeper internal invokes. This can be default safe
	// choice for native NEO/GAS (previously used on Neo 2 as "attach" mode).
	CalledByEntry WitnessScope = 0x01
	// CustomContracts define custom hash for contract-specific.
	CustomContracts WitnessScope = 0x10
	// CustomGroups define custom pubkey for group members.
	CustomGroups WitnessScope = 0x20
	// Global allows this witness in all contexts (default Neo2 behavior).
	// This cannot be combined with other flags.
	Global WitnessScope = 0x80
)

// ScopesFromString converts string of comma-separated scopes to a set of scopes
// (case-sensitive). String can combine several scopes, e.g. be any of: 'Global',
// 'CalledByEntry,CustomGroups' etc. In case of an empty string an error will be
// returned.
func ScopesFromString(s string) (WitnessScope, error) {
	var result WitnessScope
	scopes := strings.Split(s, ",")
	for i, scope := range scopes {
		scopes[i] = strings.TrimSpace(scope)
	}
	dict := map[string]WitnessScope{
		Global.String():          Global,
		CalledByEntry.String():   CalledByEntry,
		CustomContracts.String(): CustomContracts,
		CustomGroups.String():    CustomGroups,
		FeeOnly.String():         FeeOnly,
	}
	var isGlobal bool
	for _, scopeStr := range scopes {
		scope, ok := dict[scopeStr]
		if !ok {
			return result, fmt.Errorf("invalid witness scope: %v", scopeStr)
		}
		if isGlobal && !(scope == Global) {
			return result, fmt.Errorf("Global scope can not be combined with other scopes")
		}
		result |= scope
		if scope == Global {
			isGlobal = true
		}
	}
	return result, nil
}

// scopesToString converts witness scope to it's string representation. It uses
// `, ` to separate scope names.
func scopesToString(scopes WitnessScope) string {
	if scopes&Global != 0 || scopes == FeeOnly {
		return scopes.String()
	}
	var res string
	if scopes&CalledByEntry != 0 {
		res = CalledByEntry.String()
	}
	if scopes&CustomContracts != 0 {
		if len(res) != 0 {
			res += ", "
		}
		res += CustomContracts.String()
	}
	if scopes&CustomGroups != 0 {
		if len(res) != 0 {
			res += ", "
		}
		res += CustomGroups.String()
	}
	return res
}

// MarshalJSON implements json.Marshaler interface.
func (s WitnessScope) MarshalJSON() ([]byte, error) {
	return []byte(`"` + scopesToString(s) + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (s *WitnessScope) UnmarshalJSON(data []byte) error {
	var js string
	if err := json.Unmarshal(data, &js); err != nil {
		return err
	}
	scopes, err := ScopesFromString(js)
	if err != nil {
		return err
	}
	*s = scopes
	return nil
}
