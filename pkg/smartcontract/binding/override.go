package binding

import (
	"strings"
)

// Override contains a package and a type to replace manifest method parameter type with.
type Override struct {
	// Package contains a fully-qualified package name.
	Package string
	// TypeName contains type name together with a package alias.
	TypeName string
}

// NewOverrideFromString parses s and returns method parameter type override spec.
func NewOverrideFromString(s string) Override {
	var over Override

	index := strings.LastIndexByte(s, '.')
	if index == -1 {
		over.TypeName = s
		return over
	}

	// Arrays and maps can have fully-qualified types as elements.
	last := strings.LastIndexAny(s, "]*")
	isCompound := last != -1 && last < index
	if isCompound {
		over.Package = s[last+1 : index]
	} else {
		over.Package = s[:index]
	}

	switch over.Package {
	case "iterator", "storage":
		over.Package = "github.com/nspcc-dev/neo-go/pkg/interop/" + over.Package
	case "ledger", "management":
		over.Package = "github.com/nspcc-dev/neo-go/pkg/interop/native/" + over.Package
	}

	slashIndex := strings.LastIndexByte(s, '/')
	if isCompound {
		over.TypeName = s[:last+1] + s[slashIndex+1:]
	} else {
		over.TypeName = s[slashIndex+1:]
	}
	return over
}

// UnmarshalYAML implements the YAML Unmarshaler interface.
func (o *Override) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string

	err := unmarshal(&s)
	if err != nil {
		return err
	}

	*o = NewOverrideFromString(s)
	return err
}

// MarshalYAML implements the YAML marshaler interface.
func (o Override) MarshalYAML() (interface{}, error) {
	if o.Package == "" {
		return o.TypeName, nil
	}

	index := strings.LastIndexByte(o.TypeName, '.')
	last := strings.LastIndexAny(o.TypeName, "]*")
	if last == -1 {
		return o.Package + o.TypeName[index:], nil
	}
	return o.TypeName[:last+1] + o.Package + o.TypeName[index:], nil
}
