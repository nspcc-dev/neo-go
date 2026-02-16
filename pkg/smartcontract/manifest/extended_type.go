package manifest

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var (
	namedTypeRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9.]+$`)
)

// ExtendedType represents extended type metadata used in manifests, contract
// configuration and RPC bindings. It provides additional information for a
// parameter or return value.
type ExtendedType struct {
	Type       smartcontract.ParamType `json:"type" yaml:"type"`
	Name       string                  `json:"namedtype,omitempty" yaml:"namedtype,omitempty"`
	Length     uint32                  `json:"length,omitempty" yaml:"length,omitempty"`
	ForbidNull bool                    `json:"forbidnull,omitempty" yaml:"forbidnull,omitempty"`
	Interface  string                  `json:"interface,omitempty" yaml:"interface,omitempty"`
	Key        smartcontract.ParamType `json:"key,omitempty" yaml:"key,omitempty"`
	Value      *ExtendedType           `json:"value,omitempty" yaml:"value,omitempty"`
	Fields     []Parameter             `json:"fields,omitempty" yaml:"fields,omitempty"`
}

// Equals compares two extended types field-by-field and returns true if they are
// equal.
func (e *ExtendedType) Equals(other *ExtendedType) bool {
	if e == other {
		return true
	}
	if e == nil || other == nil {
		return false
	}
	if e.Type != other.Type && (e.Type != smartcontract.ByteArrayType && e.Type != smartcontract.StringType ||
		other.Type != smartcontract.ByteArrayType && other.Type != smartcontract.StringType) {
		return false
	}
	if e.Name != other.Name || e.Interface != other.Interface ||
		e.Key != other.Key || e.Length != other.Length || e.ForbidNull != other.ForbidNull {
		return false
	}
	if len(e.Fields) != len(other.Fields) {
		return false
	}
	for i := range e.Fields {
		a := &e.Fields[i]
		b := &other.Fields[i]
		if a.Name != b.Name {
			return false
		}
		if a.Type != b.Type && (a.Type != smartcontract.ByteArrayType && a.Type != smartcontract.StringType ||
			b.Type != smartcontract.ByteArrayType && b.Type != smartcontract.StringType) {
			return false
		}
		if !a.ExtendedType.Equals(b.ExtendedType) {
			return false
		}
	}
	return (e.Value == nil && other.Value == nil) || (e.Value != nil && other.Value != nil && e.Value.Equals(other.Value))
}

func (e *ExtendedType) ToStackItem() stackitem.Item {
	m := stackitem.NewMap()
	m.Add(stackitem.Make("type"), stackitem.Make(int(e.Type)))
	if e.Name != "" {
		m.Add(stackitem.Make("namedtype"), stackitem.Make(e.Name))
	}
	if e.Length != 0 {
		m.Add(stackitem.Make("length"), stackitem.Make(int(e.Length)))
	}
	if e.ForbidNull {
		m.Add(stackitem.Make("forbidnull"), stackitem.Make(true))
	}
	if e.Interface != "" {
		m.Add(stackitem.Make("interface"), stackitem.Make(e.Interface))
	}
	if e.Key != smartcontract.AnyType {
		m.Add(stackitem.Make("key"), stackitem.Make(int(e.Key)))
	}
	if e.Value != nil {
		m.Add(stackitem.Make("value"), e.Value.ToStackItem())
	}
	if e.Fields != nil {
		fields := make([]stackitem.Item, len(e.Fields))
		for i := range e.Fields {
			fields[i] = e.Fields[i].ToStackItem()
		}
		m.Add(stackitem.Make("fields"), stackitem.NewArray(fields))
	}
	return m
}

func (e *ExtendedType) FromStackItem(item stackitem.Item) error {
	if item == nil {
		return errors.New("expected non-nil item")
	}
	if item.Type() != stackitem.MapT {
		return errors.New("invalid ExtendedType stackitem type")
	}
	raw := item.Value().([]stackitem.MapElement)
	m := make(map[string]stackitem.Item, len(raw))
	for _, mElem := range raw {
		ks, err := stackitem.ToString(mElem.Key)
		if err != nil {
			continue
		}
		m[ks] = mElem.Value
	}
	ti, ok := m["type"]
	if !ok {
		return errors.New("incorrect type")
	}
	if ti.Type() != stackitem.IntegerT {
		return errors.New("type must be integer")
	}
	e.Type = smartcontract.ParamType(ti.Value().(*big.Int).Int64())
	nti, ok := m["namedtype"]
	if ok {
		var err error
		if e.Name, err = stackitem.ToString(nti); err != nil {
			return fmt.Errorf("can't get namedtype: %w", err)
		}
	} else {
		e.Name = ""
	}
	li, ok := m["length"]
	if ok {
		if li.Type() != stackitem.IntegerT {
			return errors.New("length must be integer or null")
		}
		e.Length = uint32(li.Value().(*big.Int).Uint64())
	} else {
		e.Length = 0
	}
	fni, ok := m["forbidnull"]
	if ok {
		if fni.Type() != stackitem.BooleanT {
			return errors.New("forbidnull must be boolean or null")
		}
		e.ForbidNull = fni.Value().(bool)
	} else {
		e.ForbidNull = false
	}
	ii, ok := m["interface"]
	if ok {
		if ii.Type() != stackitem.ByteArrayT {
			return errors.New("interface must be bytearray or null")
		}
		e.Interface = string(ii.Value().([]byte))
	} else {
		e.Interface = ""
	}
	ki, ok := m["key"]
	if ok {
		if ki.Type() != stackitem.IntegerT {
			return errors.New("key must be integer or null")
		}
		e.Key = smartcontract.ParamType(ki.Value().(*big.Int).Int64())
	} else {
		e.Key = smartcontract.AnyType
	}
	vi, ok := m["value"]
	if ok {
		e.Value = new(ExtendedType)
		if err := e.Value.FromStackItem(vi); err != nil {
			return fmt.Errorf("can't get value: %w", err)
		}
	} else {
		e.Value = nil
	}
	fi, ok := m["fields"]
	if ok {
		if fi.Type() != stackitem.ArrayT {
			return errors.New("fields must be array or null")
		}
		fields := fi.Value().([]stackitem.Item)
		e.Fields = make([]Parameter, len(fields))
		for i := range fields {
			if err := e.Fields[i].FromStackItem(fields[i]); err != nil {
				return err
			}
		}
	} else {
		e.Fields = nil
	}
	return e.IsValid()
}

func (e *ExtendedType) IsValid() error {
	if _, err := smartcontract.ConvertToParamType(int(e.Type)); err != nil {
		return err
	}
	if e.Name != "" {
		if e.Type != smartcontract.ArrayType {
			return fmt.Errorf("`ExtendedType.Name` field can not be specified for %s", e.Type)
		}
		if len(e.Name) > 64 {
			return errors.New("`ExtendedType.Name` must not be longer than 64 characters")
		}
		if !namedTypeRE.MatchString(e.Name) {
			return errors.New("`ExtendedType.Name` must start with a letter and contain only letters, digits and dots")
		}
	}
	if e.Length != 0 {
		switch e.Type {
		case smartcontract.IntegerType, smartcontract.ByteArrayType,
			smartcontract.StringType, smartcontract.ArrayType:
		default:
			return fmt.Errorf("`ExtendedType.Length` field can not be specified for %s", e.Type)
		}
	}
	if e.ForbidNull {
		switch e.Type {
		case smartcontract.Hash160Type, smartcontract.Hash256Type, smartcontract.ByteArrayType,
			smartcontract.StringType, smartcontract.ArrayType, smartcontract.MapType,
			smartcontract.InteropInterfaceType:
		default:
			return fmt.Errorf("`ExtendedType.ForbidNull` field can not be specified for %s", e.Type)
		}
	}
	if e.Interface != "" {
		if e.Type != smartcontract.InteropInterfaceType {
			return fmt.Errorf("`ExtendedType.Interface` field can not be specified for %s", e.Type)
		}
		if e.Interface != stackitem.IteratorInterfaceName {
			return fmt.Errorf("invalid value for `ExtendedType.Interface` field: %s", e.Interface)
		}
	} else if e.Type == smartcontract.InteropInterfaceType {
		return fmt.Errorf("`ExtendedType.Interface` field is required for %s", e.Type)
	}
	if e.Key != smartcontract.AnyType {
		if e.Type != smartcontract.MapType {
			return fmt.Errorf("`ExtendedType.Key` field can not be specified for %s", e.Type)
		}
		switch e.Key {
		case smartcontract.SignatureType, smartcontract.BoolType, smartcontract.IntegerType,
			smartcontract.Hash160Type, smartcontract.Hash256Type, smartcontract.ByteArrayType,
			smartcontract.PublicKeyType, smartcontract.StringType:
		default:
			return fmt.Errorf("key %s is not allowed for map definitions", e.Key)
		}
	} else if e.Type == smartcontract.MapType {
		return fmt.Errorf("`ExtendedType.Key` field is required for %s", e.Type)
	}
	if e.Value != nil {
		switch e.Type {
		case smartcontract.ArrayType, smartcontract.InteropInterfaceType, smartcontract.MapType:
			if err := e.Value.IsValid(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("`ExtendedType.Value` field can not be specified for %s", e.Type)
		}
	} else {
		if e.Type == smartcontract.ArrayType && e.Name == "" {
			return fmt.Errorf("`ExtendedType.Value` field is required for %s", e.Type)
		}
		switch e.Type {
		case smartcontract.InteropInterfaceType, smartcontract.MapType:
			return fmt.Errorf("`ExtendedType.Value` field is required for %s", e.Type)
		default:
		}
	}
	if e.Fields != nil {
		if e.Type != smartcontract.ArrayType {
			return fmt.Errorf("`ExtendedType.Fields` field can not be specified for %s", e.Type)
		}
		for i := range e.Fields {
			if err := e.Fields[i].IsValid(); err != nil {
				return err
			}
		}
	}
	return nil
}
