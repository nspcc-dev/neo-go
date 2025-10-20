package manifest

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"gopkg.in/yaml.v3"
)

type ExtendedType struct {
	Base       smartcontract.ParamType `json:"type" yaml:"type"`
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
	if e.Base != other.Base && (e.Base != smartcontract.ByteArrayType && e.Base != smartcontract.StringType ||
		other.Base != smartcontract.ByteArrayType && other.Base != smartcontract.StringType) {
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
		pa := &e.Fields[i]
		pb := &other.Fields[i]
		if pa.Name != pb.Name {
			return false
		}
		if pa.Type != pb.Type && (pa.Type != smartcontract.ByteArrayType && pa.Type != smartcontract.StringType ||
			pb.Type != smartcontract.ByteArrayType && pb.Type != smartcontract.StringType) {
			return false
		}
		if !pa.ExtendedType.Equals(pb.ExtendedType) {
			return false
		}
	}
	return (e.Value == nil && other.Value == nil) || (e.Value != nil && other.Value != nil && e.Value.Equals(other.Value))
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (e *ExtendedType) UnmarshalYAML(node *yaml.Node) error {
	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if v, ok := raw["base"]; ok {
		if _, ok := raw["type"]; !ok {
			raw["type"] = v
		}
		delete(raw, "base")
	}
	if v, ok := raw["name"]; ok {
		if _, ok := raw["namedtype"]; !ok {
			raw["namedtype"] = v
		}
		delete(raw, "name")
	}
	b, _ := yaml.Marshal(raw)
	type ext ExtendedType
	var tmp ext
	if err := yaml.Unmarshal(b, &tmp); err != nil {
		return err
	}
	*e = ExtendedType(tmp)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *ExtendedType) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["base"]; ok {
		if _, ok := raw["type"]; !ok {
			raw["type"] = v
		}
		delete(raw, "base")
	}
	if v, ok := raw["name"]; ok {
		if _, ok := raw["namedtype"]; !ok {
			raw["namedtype"] = v
		}
		delete(raw, "name")
	}
	b, _ := json.Marshal(raw)
	type ext ExtendedType
	var tmp ext
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	*e = ExtendedType(tmp)
	return nil
}

func (e *ExtendedType) ToStackItem() stackitem.Item {
	var v stackitem.Item
	if e.Value != nil {
		v = e.Value.ToStackItem()
	}
	var fieldsItem stackitem.Item
	if e.Fields != nil {
		fields := make([]stackitem.Item, len(e.Fields))
		for i := range e.Fields {
			fields[i] = e.Fields[i].ToStackItem()
		}
		fieldsItem = stackitem.NewArray(fields)
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(int(e.Base)),
		stackitem.Make(e.Name),
		stackitem.Make(e.Length),
		stackitem.Make(e.ForbidNull),
		stackitem.Make(e.Interface),
		stackitem.Make(int(e.Key)),
		stackitem.Make(v),
		stackitem.Make(fieldsItem),
	})
}

func (e *ExtendedType) FromStackItem(item stackitem.Item) error {
	if item == nil {
		return errors.New("expected non-nil item")
	}
	if item.Type() != stackitem.StructT {
		return errors.New("invalid ExtendedType stackitem type")
	}
	arr := item.Value().([]stackitem.Item)
	if len(arr) != 8 {
		return errors.New("invalid ExtendedType stackitem length")
	}
	bi, err := arr[0].TryInteger()
	if err != nil {
		return fmt.Errorf("can't get ExtendedType.Base: %w", err)
	}
	e.Base = smartcontract.ParamType(int(bi.Int64()))
	e.Name, err = stackitem.ToString(arr[1])
	if err != nil {
		return fmt.Errorf("can't get ExtendedType.Name: %w", err)
	}
	li, err := arr[2].TryInteger()
	if err != nil {
		return fmt.Errorf("can't get ExtendedType.Length: %w", err)
	}
	e.Length = uint32(li.Uint64())
	fb, err := arr[3].TryBool()
	if err != nil {
		return fmt.Errorf("can't get ExtendedType.ForbidNull: %w", err)
	}
	e.ForbidNull = fb
	e.Interface, err = stackitem.ToString(arr[4])
	if err != nil {
		return fmt.Errorf("can't get ExtendedType.Interface: %w", err)
	}
	ki, err := arr[5].TryInteger()
	if err != nil {
		return fmt.Errorf("can't get ExtendedType.Key: %w", err)
	}
	e.Key = smartcontract.ParamType(int(ki.Int64()))
	if arr[6].Value() == nil {
		e.Value = nil
	} else {
		et := new(ExtendedType)
		if err := et.FromStackItem(arr[6]); err != nil {
			return fmt.Errorf("can't get ExtendedType.Value: %w", err)
		}
		e.Value = et
	}
	if arr[7].Value() == nil {
		return nil
	}
	if arr[7].Type() != stackitem.ArrayT {
		return errors.New("invalid ExtendedType fields stackitem type")
	}
	fields := arr[7].Value().([]stackitem.Item)
	e.Fields = make([]Parameter, len(fields))
	for i := range fields {
		var p Parameter
		if err := p.FromStackItem(fields[i]); err != nil {
			return err
		}
		e.Fields[i] = p
	}
	return nil
}

func (e *ExtendedType) IsValid() error {
	if _, err := smartcontract.ConvertToParamType(int(e.Base)); err != nil {
		return err
	}
	if e.Name != "" && e.Base != smartcontract.ArrayType {
		return fmt.Errorf("`ExtendedType.Name` field can not be specified for %s", e.Base)
	}
	if e.Length != 0 {
		switch e.Base {
		case smartcontract.IntegerType, smartcontract.ByteArrayType,
			smartcontract.StringType, smartcontract.ArrayType:
		default:
			return fmt.Errorf("`ExtendedType.Length` field can not be specified for %s", e.Base)
		}
	}
	if e.ForbidNull {
		switch e.Base {
		case smartcontract.Hash160Type, smartcontract.Hash256Type, smartcontract.ByteArrayType,
			smartcontract.StringType, smartcontract.ArrayType, smartcontract.MapType,
			smartcontract.InteropInterfaceType:
		default:
			return fmt.Errorf("`ExtendedType.ForbidNull` field can not be specified for %s", e.Base)
		}
	}
	if e.Interface != "" {
		if e.Base != smartcontract.InteropInterfaceType {
			return fmt.Errorf("`ExtendedType.Interface` field can not be specified for %s", e.Base)
		}
		if e.Interface != "IIterator" {
			return fmt.Errorf("invalid value for `ExtendedType.Interface` field: %s", e.Interface)
		}
	} else if e.Base == smartcontract.InteropInterfaceType {
		return fmt.Errorf("`ExtendedType.Interface` field is required for %s", e.Base)
	}
	if e.Key != smartcontract.AnyType {
		if e.Base != smartcontract.MapType {
			return fmt.Errorf("`ExtendedType.Key` field can not be specified for %s", e.Base)
		}
		switch e.Key {
		case smartcontract.SignatureType, smartcontract.BoolType, smartcontract.IntegerType,
			smartcontract.Hash160Type, smartcontract.Hash256Type, smartcontract.ByteArrayType,
			smartcontract.PublicKeyType, smartcontract.StringType:
		default:
			return fmt.Errorf("key %s is not allowed for map definitions", e.Key)
		}
	} else if e.Base == smartcontract.MapType {
		return fmt.Errorf("`ExtendedType.Key` field is required for %s", e.Base)
	}
	if e.Value != nil {
		switch e.Base {
		case smartcontract.ArrayType, smartcontract.InteropInterfaceType, smartcontract.MapType:
			if err := e.Value.IsValid(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("`ExtendedType.Value` field can not be specified for %s", e.Base)
		}
	} else {
		if e.Base == smartcontract.ArrayType && e.Name == "" {
			return fmt.Errorf("`ExtendedType.Value` field is required for %s", e.Base)
		}
		switch e.Base {
		case smartcontract.InteropInterfaceType, smartcontract.MapType:
			return fmt.Errorf("`ExtendedType.Value` field is required for %s", e.Base)
		default:
		}
	}
	if e.Fields != nil {
		if e.Base != smartcontract.ArrayType {
			return fmt.Errorf("`ExtendedType.Fields` field can not be specified for %s", e.Base)
		}
		for i := range e.Fields {
			if err := e.Fields[i].IsValid(); err != nil {
				return err
			}
		}
	}
	return nil
}
