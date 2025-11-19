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
	ErrInvalidParameterName  = errors.New("invalid parameter name")
	ErrInvalidParameterType  = errors.New("invalid parameter type")
	ErrSafeMethodMismatch    = errors.New("method has wrong safe flag")
)

type extendedStandard struct {
	// extendedName is a user-facing name of the standard in case if this
	// standard is a sub-standard of some base standard.
	extendedName string
	*Standard
}

var checks = map[string][]extendedStandard{
	manifest.NEP11StandardName: {
		extendedStandard{
			extendedName: "non-divisible",
			Standard:     Nep11NonDivisible,
		},
		extendedStandard{
			extendedName: "divisible",
			Standard:     Nep11Divisible,
		}},
	manifest.NEP17StandardName: {{Standard: Nep17}},
	manifest.NEP22StandardName: {{Standard: Nep22}},
	manifest.NEP26StandardName: {{Standard: Nep26}},
	manifest.NEP27StandardName: {{Standard: Nep27}},
	manifest.NEP24StandardName: {{Standard: Nep24}},
	manifest.NEP24Payable:      {{Standard: Nep24Payable}},
	manifest.NEP29StandardName: {{Standard: Nep29}},
	manifest.NEP30StandardName: {{Standard: Nep30}},
	manifest.NEP31StandardName: {{Standard: Nep31}},
}

// Check checks if the manifest complies with all provided standards.
// If the standard method's parameters slice is nil, no parameters check is
// conducted. To enforce zero parameters check, the standard method's parameters
// slice must be set to an empty list.
func Check(m *manifest.Manifest, standards ...string) error {
	return check(m, true, standards...)
}

// CheckABI is similar to Check but doesn't check parameter names.
func CheckABI(m *manifest.Manifest, standards ...string) error {
	return check(m, false, standards...)
}

func check(m *manifest.Manifest, checkNames bool, standards ...string) error {
	for i := range standards {
		ss, ok := checks[standards[i]]
		if ok {
			var errs []error
			for i := range ss {
				var err error
				if err = comply(m, checkNames, ss[i].Standard); err == nil {
					errs = nil
					break
				}
				if len(ss) > 1 {
					err = fmt.Errorf("%s: %w", ss[i].extendedName, err)
				}
				errs = append(errs, err)
			}
			if len(errs) > 0 {
				err := fmt.Errorf("manifest is not compliant with '%s': %w", standards[i], errs[0])
				for _, e := range errs[1:] {
					err = fmt.Errorf("%w; %w", err, e)
				}
				return err
			}
		}
	}
	return nil
}

// Comply if m has all methods and event from st manifest and they have the same signature.
// Parameter names are checked to exactly match the ones in the given standard.
// If the standard method's parameters slice is nil, no parameters check is
// conducted. To enforce zero parameters check, the standard method's parameters
// slice must be set to an empty list.
func Comply(m *manifest.Manifest, st *Standard) error {
	return comply(m, true, st)
}

// ComplyABI is similar to Comply but doesn't check parameter names.
func ComplyABI(m *manifest.Manifest, st *Standard) error {
	return comply(m, false, st)
}

func comply(m *manifest.Manifest, checkNames bool, st *Standard) error {
	if len(st.Required) > 0 {
		if err := check(m, checkNames, st.Required...); err != nil {
			return fmt.Errorf("required standard '%s' is not supported: %w", st.Name, err)
		}
	}
	if st.Base != nil {
		if err := comply(m, checkNames, st.Base); err != nil {
			return err
		}
	}
	for _, stm := range st.ABI.Methods {
		if err := checkMethod(m, &stm, false, checkNames); err != nil {
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
			if checkNames && ste.Parameters[i].Name != ed.Parameters[i].Name {
				return fmt.Errorf("%w: event '%s'[%d] (expected %s, got %s)", ErrInvalidParameterName,
					name, i, ste.Parameters[i].Name, ed.Parameters[i].Name)
			}
			if ste.Parameters[i].Type != ed.Parameters[i].Type {
				return fmt.Errorf("%w: event '%s' (expected %s, got %s)", ErrInvalidParameterType,
					name, ste.Parameters[i].Type, ed.Parameters[i].Type)
			}
		}
	}
	for _, stm := range st.Optional {
		if err := checkMethod(m, &stm, true, checkNames); err != nil {
			return err
		}
	}
	return nil
}

// checkMethod checks whether the method with the specified parameters count is
// included into manifest and matches the expected signature. It returns an error
// in case if the method is missing and allowMissing is not set or in case if
// parameter names don't match the expected ones and checkNames is set. If expected
// parameters slice is nil, no parameters check is conducted. Set expected
// parameters slice to an empty list to enforce zero parameters check.
func checkMethod(m *manifest.Manifest, expected *manifest.Method,
	allowMissing bool, checkNames bool) error {
	var expectedParamsLen = -1
	if expected.Parameters != nil {
		expectedParamsLen = len(expected.Parameters)
	}
	actual := m.ABI.GetMethod(expected.Name, expectedParamsLen)
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
		if checkNames && expected.Parameters[i].Name != actual.Parameters[i].Name {
			return fmt.Errorf("%w: '%s'[%d] (expected %s, got %s)", ErrInvalidParameterName,
				expected.Name, i, expected.Parameters[i].Name, actual.Parameters[i].Name)
		}
		if expected.Parameters[i].Type != actual.Parameters[i].Type {
			return fmt.Errorf("%w: '%s'[%d] (expected %s, got %s)", ErrInvalidParameterType,
				expected.Name, i, expected.Parameters[i].Type, actual.Parameters[i].Type)
		}
	}
	if expected.Safe != actual.Safe {
		return fmt.Errorf("'%s' %w: expected %t", expected.Name, ErrSafeMethodMismatch, expected.Safe)
	}
	return nil
}
