package transaction

//go:generate stringer -type=WitnessScope -linecomment -output=witness_scope_string.go
import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// WitnessScope represents set of witness flags for Transaction signer.
type WitnessScope byte

const (
	// None specifies that no contract was witnessed. Only sign the transaction.
	None WitnessScope = 0
	// CalledByEntry witness is only valid in entry script and ones directly called by it.
	// No params is needed, as the witness/permission/signature given on first invocation will
	// automatically expire if entering deeper internal invokes. This can be default safe
	// choice for native NEO/GAS (previously used on Neo 2 as "attach" mode).
	CalledByEntry WitnessScope = 0x01
	// CustomContracts define custom hash for contract-specific.
	CustomContracts WitnessScope = 0x10
	// CustomGroups define custom pubkey for group members.
	CustomGroups WitnessScope = 0x20
	// Rules is a set of conditions with boolean operators.
	Rules WitnessScope = 0x40 // WitnessRules
	// Global allows this witness in all contexts (default Neo2 behavior).
	// This cannot be combined with other flags.
	Global WitnessScope = 0x80
)

// ScopesFromByte converts byte to a set of WitnessScopes and performs validity
// check.
func ScopesFromByte(b byte) (WitnessScope, error) {
	var res = WitnessScope(b)
	if (res&Global != 0) && (res&(None|CalledByEntry|CustomContracts|CustomGroups|Rules) != 0) {
		return 0, errors.New("Global scope can not be combined with other scopes")
	}
	if res&^(None|CalledByEntry|CustomContracts|CustomGroups|Rules|Global) != 0 {
		return 0, fmt.Errorf("invalid scope %d", res)
	}
	return res, nil
}

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
		Rules.String():           Rules,
		None.String():            None,
	}
	var isGlobal bool
	for _, scopeStr := range scopes {
		scope, ok := dict[scopeStr]
		if !ok {
			return result, fmt.Errorf("invalid witness scope: %v", scopeStr)
		}
		if isGlobal && !(scope == Global) {
			return result, errors.New("Global scope can not be combined with other scopes")
		}
		result |= scope
		if scope == Global {
			isGlobal = true
		}
	}
	return result, nil
}

func appendScopeString(str string, scopes WitnessScope, scope WitnessScope) string {
	if scopes&scope != 0 {
		if len(str) != 0 {
			str += ", "
		}
		str += scope.String()
	}
	return str
}

// scopesToString converts witness scope to it's string representation. It uses
// `, ` to separate scope names.
func scopesToString(scopes WitnessScope) string {
	if scopes&Global != 0 || scopes == None {
		return scopes.String()
	}
	var res string
	res = appendScopeString(res, scopes, CalledByEntry)
	res = appendScopeString(res, scopes, CustomContracts)
	res = appendScopeString(res, scopes, CustomGroups)
	res = appendScopeString(res, scopes, Rules)
	return res
}

// MarshalJSON implements the json.Marshaler interface.
func (s WitnessScope) MarshalJSON() ([]byte, error) {
	return []byte(`"` + scopesToString(s) + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
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
