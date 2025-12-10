package manifest

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"gopkg.in/yaml.v3"
)

// Parameter represents smartcontract's parameter's definition.
type Parameter struct {
	Name         string                  `json:"name"`
	Type         smartcontract.ParamType `json:"type"`
	ExtendedType *ExtendedType           `json:"extendedtype,omitempty" yaml:"extendedtype,omitempty"`
}

// Parameters is just an array of Parameter.
type Parameters []Parameter

// NewParameter returns a new parameter of the specified name and type.
func NewParameter(name string, typ smartcontract.ParamType) Parameter {
	return Parameter{
		Name: name,
		Type: typ,
	}
}

// IsValid checks Parameter consistency and correctness.
func (p *Parameter) IsValid() error {
	if p.Name == "" {
		return errors.New("empty or absent name")
	}
	if p.Type == smartcontract.VoidType {
		return errors.New("void parameter")
	}
	if p.ExtendedType != nil {
		if err := p.ExtendedType.IsValid(); err != nil {
			return err
		}
	}
	_, err := smartcontract.ConvertToParamType(int(p.Type))
	return err
}

// ToStackItem converts Parameter to stackitem.Item.
func (p *Parameter) ToStackItem() stackitem.Item {
	items := []stackitem.Item{
		stackitem.Make(p.Name),
		stackitem.Make(int(p.Type)),
	}
	if p.ExtendedType != nil {
		items = append(items, p.ExtendedType.ToStackItem())
	}
	return stackitem.NewStruct(items)
}

// FromStackItem converts stackitem.Item to Parameter.
func (p *Parameter) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Parameter stackitem type")
	}
	param := item.Value().([]stackitem.Item)
	if len(param) < 2 {
		return errors.New("invalid Parameter stackitem length")
	}
	p.Name, err = stackitem.ToString(param[0])
	if err != nil {
		return err
	}
	typ, err := param[1].TryInteger()
	if err != nil {
		return err
	}
	p.Type, err = smartcontract.ConvertToParamType(int(typ.Int64()))
	if err != nil {
		return err
	}
	if len(param) == 2 || param[2].Value() == nil {
		return nil
	}
	p.ExtendedType = &ExtendedType{}
	if err = p.ExtendedType.FromStackItem(param[2]); err != nil {
		return err
	}
	return nil
}

// AreValid checks all parameters for validity and consistency.
func (p Parameters) AreValid() error {
	for i := range p {
		err := p[i].IsValid()
		if err != nil {
			return fmt.Errorf("parameter #%d/%q: %w", i, p[i].Name, err)
		}
	}
	if sliceHasDups(p, func(a, b Parameter) int {
		return cmp.Compare(a.Name, b.Name)
	}) {
		return errors.New("duplicate parameter name")
	}
	return nil
}

// sliceHasDups checks the slice for duplicate elements.
func sliceHasDups[S ~[]E, E any](x S, cmp func(a, b E) int) bool {
	if len(x) < 2 {
		return false
	}
	if len(x) > 2 {
		x = slices.Clone(x)
		slices.SortFunc(x, cmp)
	}
	for i := range x {
		if i == 0 {
			continue
		}
		if cmp(x[i-1], x[i]) == 0 {
			return true
		}
	}
	return false
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (p *Parameter) UnmarshalYAML(node *yaml.Node) error {
	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if v, ok := raw["field"]; ok {
		if _, ok := raw["name"]; !ok {
			raw["name"] = v
		}
		delete(raw, "field")
	}
	if _, ok := raw["extendedtype"]; !ok {
		m := make(map[string]any)
		for k, v := range raw {
			if k != "type" && k != "name" {
				m[k] = v
				delete(raw, k)
			}
		}
		if len(m) > 0 {
			raw["extendedtype"] = m
		}
	}
	var tmpET *ExtendedType
	if et, ok := raw["extendedtype"]; ok {
		b, _ := yaml.Marshal(et)
		tmpET = &ExtendedType{}
		if err := yaml.Unmarshal(b, tmpET); err != nil {
			return err
		}
		delete(raw, "extendedtype")
		if _, ok := raw["type"]; !ok {
			raw["type"] = tmpET.Type.String()
		}
	}
	b, _ := yaml.Marshal(raw)
	type param Parameter
	var tmpParam param
	if err := yaml.Unmarshal(b, &tmpParam); err != nil {
		return err
	}
	if tmpET != nil && tmpParam.Type != tmpET.Type {
		return fmt.Errorf("conflicting types: parameter type %v vs extendedtype type %v", tmpParam.Type, tmpET.Type)
	}
	*p = Parameter(tmpParam)
	p.ExtendedType = tmpET
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *Parameter) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["field"]; ok {
		if _, ok := raw["name"]; !ok {
			raw["name"] = v
		}
		delete(raw, "field")
	}
	if _, ok := raw["extendedtype"]; !ok {
		m := make(map[string]json.RawMessage)
		for k, v := range raw {
			if k != "type" && k != "name" {
				m[k] = v
				delete(raw, k)
			}
		}
		if len(m) > 0 {
			b, err := json.Marshal(m)
			if err != nil {
				return err
			}
			raw["extendedtype"] = b
		}
	}
	var tmpET *ExtendedType
	if etRaw, ok := raw["extendedtype"]; ok {
		tmpET = &ExtendedType{}
		if err := json.Unmarshal(etRaw, tmpET); err != nil {
			return err
		}
		delete(raw, "extendedtype")
		if _, ok := raw["type"]; !ok {
			raw["type"] = json.RawMessage(tmpET.Type.String())
		}
	}
	b, _ := json.Marshal(raw)
	type param Parameter
	var tmpParam param
	if err := json.Unmarshal(b, &tmpParam); err != nil {
		return err
	}
	if tmpET != nil && tmpParam.Type != tmpET.Type {
		return fmt.Errorf("conflicting types: parameter type %v vs extendedtype type %v", tmpParam.Type, tmpET.Type)
	}
	*p = Parameter(tmpParam)
	p.ExtendedType = tmpET
	return nil
}
