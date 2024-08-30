package manifest

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Parameter represents smartcontract's parameter's definition.
type Parameter struct {
	Name string                  `json:"name"`
	Type smartcontract.ParamType `json:"type"`
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
	_, err := smartcontract.ConvertToParamType(int(p.Type))
	return err
}

// ToStackItem converts Parameter to stackitem.Item.
func (p *Parameter) ToStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(p.Name),
		stackitem.Make(int(p.Type)),
	})
}

// FromStackItem converts stackitem.Item to Parameter.
func (p *Parameter) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Parameter stackitem type")
	}
	param := item.Value().([]stackitem.Item)
	if len(param) != 2 {
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
