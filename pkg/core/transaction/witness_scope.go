package transaction

import (
	"fmt"
	"strings"
)

// WitnessScope represents set of witness flags for Transaction cosigner.
type WitnessScope byte

const (
	// Global allows this witness in all contexts (default Neo2 behavior).
	// This cannot be combined with other flags.
	Global WitnessScope = 0x00
	// CalledByEntry means that this condition must hold: EntryScriptHash == CallingScriptHash.
	// No params is needed, as the witness/permission/signature given on first invocation will
	// automatically expire if entering deeper internal invokes. This can be default safe
	// choice for native NEO/GAS (previously used on Neo 2 as "attach" mode).
	CalledByEntry WitnessScope = 0x01
	// CustomContracts define custom hash for contract-specific.
	CustomContracts WitnessScope = 0x10
	// CustomGroups define custom pubkey for group members.
	CustomGroups WitnessScope = 0x20
)

// ScopesFromString converts string of comma-separated scopes to a set of scopes
// (case doesn't matter). String can combine several scopes, e.g. be any of:
// 'Global', 'CalledByEntry,CustomGroups' etc. In case of an empty string an
// error will be returned.
func ScopesFromString(s string) (WitnessScope, error) {
	var result WitnessScope
	s = strings.ToLower(s)
	scopes := strings.Split(s, ",")
	dict := map[string]WitnessScope{
		"global":          Global,
		"calledbyentry":   CalledByEntry,
		"customcontracts": CustomContracts,
		"customgroups":    CustomGroups,
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
