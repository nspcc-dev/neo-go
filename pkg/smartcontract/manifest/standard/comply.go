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

var checks = map[string][]*Standard{
	manifest.NEP11StandardName: {nep11NonDivisible, nep11Divisible},
	manifest.NEP17StandardName: {nep17},
}

// Check checks if manifest complies with all provided standards.
// Currently only NEP-17 is supported.
func Check(m *manifest.Manifest, standards ...string) error {
	for i := range standards {
		ss, ok := checks[standards[i]]
		if ok {
			var err error
			for i := range ss {
				if err = Comply(m, ss[i]); err == nil {
					break
				}
			}
			if err != nil {
				return fmt.Errorf("manifest is not compliant with '%s': %w", standards[i], err)
			}
		}
	}
	return nil
}

// Comply if m has all methods and event from st manifest and they have the same signature.
// Parameter names are ignored.
func Comply(m *manifest.Manifest, st *Standard) error {
	if st.Base != nil {
		if err := Comply(m, st.Base); err != nil {
			return err
		}
	}
	for _, stm := range st.ABI.Methods {
		if err := checkMethod(m, &stm, false); err != nil {
			return err
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
	for _, stm := range st.Optional {
		if err := checkMethod(m, &stm, true); err != nil {
			return err
		}
	}
	return nil
}

func checkMethod(m *manifest.Manifest, expected *manifest.Method, allowMissing bool) error {
	actual := m.ABI.GetMethod(expected.Name, len(expected.Parameters))
	if actual == nil {
		if allowMissing {
			return nil
		}
		return fmt.Errorf("%w: '%s' with %d parameters", ErrMethodMissing,
			expected.Name, len(expected.Parameters))
	}
	if expected.ReturnType != actual.ReturnType {
		return fmt.Errorf("%w: '%s' (expected %s, got %s)", ErrInvalidReturnType,
			expected.Name, expected.ReturnType, actual.ReturnType)
	}
	for i := range expected.Parameters {
		if expected.Parameters[i].Type != actual.Parameters[i].Type {
			return fmt.Errorf("%w: '%s'[%d] (expected %s, got %s)", ErrInvalidParameterType,
				expected.Name, i, expected.Parameters[i].Type, actual.Parameters[i].Type)
		}
	}
	if expected.Safe != actual.Safe {
		return fmt.Errorf("%w: expected %t", ErrSafeMethodMismatch, expected.Safe)
	}
	return nil
}
