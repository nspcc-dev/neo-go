package standard

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Various validation errors.
var (
	ErrMethodMissing         = errors.New("method missing")
	ErrEventMissing          = errors.New("event missing")
	ErrInvalidReturnType     = errors.New("invalid return type")
	ErrInvalidParameterCount = errors.New("invalid parameter count")
	ErrInvalidParameterType  = errors.New("invalid parameter type")
	ErrSafeMethodMismatch    = errors.New("method has wrong safe flag")
)

var checks = map[string]*manifest.Manifest{
	manifest.NEP17StandardName: nep17,
}

// Check checks if manifest complies with all provided standards.
// Currently only NEP-17 is supported.
func Check(m *manifest.Manifest, standards ...string) error {
	for i := range standards {
		s, ok := checks[standards[i]]
		if ok {
			if err := Comply(m, s); err != nil {
				return fmt.Errorf("manifest is not compliant with '%s': %w", standards[i], err)
			}
		}
	}
	return nil
}

// Comply if m has all methods and event from st manifest and they have the same signature.
// Parameter names are ignored.
func Comply(m, st *manifest.Manifest) error {
	for _, stm := range st.ABI.Methods {
		name := stm.Name
		md := m.ABI.GetMethod(name, len(stm.Parameters))
		if md == nil {
			return fmt.Errorf("%w: '%s' with %d parameters", ErrMethodMissing, name, len(stm.Parameters))
		} else if stm.ReturnType != md.ReturnType {
			return fmt.Errorf("%w: '%s' (expected %s, got %s)", ErrInvalidReturnType,
				name, stm.ReturnType, md.ReturnType)
		}
		for i := range stm.Parameters {
			if stm.Parameters[i].Type != md.Parameters[i].Type {
				return fmt.Errorf("%w: '%s'[%d] (expected %s, got %s)", ErrInvalidParameterType,
					name, i, stm.Parameters[i].Type, md.Parameters[i].Type)
			}
		}
		if stm.Safe != md.Safe {
			return fmt.Errorf("%w: expected %t", ErrSafeMethodMismatch, stm.Safe)
		}
	}
	for _, ste := range st.ABI.Events {
		name := ste.Name
		ed := m.ABI.GetEvent(name)
		if ed == nil {
			return fmt.Errorf("%w: event '%s'", ErrEventMissing, name)
		} else if len(ste.Parameters) != len(ed.Parameters) {
			return fmt.Errorf("%w: event '%s' (expected %d, got %d)", ErrInvalidParameterCount,
				name, len(ste.Parameters), len(ed.Parameters))
		}
		for i := range ste.Parameters {
			if ste.Parameters[i].Type != ed.Parameters[i].Type {
				return fmt.Errorf("%w: event '%s' (expected %s, got %s)", ErrInvalidParameterType,
					name, ste.Parameters[i].Type, ed.Parameters[i].Type)
			}
		}
	}
	return nil
}
