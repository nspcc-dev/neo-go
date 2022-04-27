package manifest

import (
	"errors"
	"sort"

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
			return err
		}
	}
	if len(p) < 2 {
		return nil
	}
	names := make([]string, len(p))
	for i := range p {
		names[i] = p[i].Name
	}
	if stringsHaveDups(names) {
		return errors.New("duplicate parameter name")
	}
	return nil
}

// stringsHaveDups checks the given set of strings for duplicates. It modifies the slice given!
func stringsHaveDups(strings []string) bool {
	sort.Strings(strings)
	for i := range strings {
		if i == 0 {
			continue
		}
		if strings[i] == strings[i-1] {
			return true
		}
	}
	return false
}

// permissionDescsHaveDups checks the given set of strings for duplicates. It modifies the slice given!
func permissionDescsHaveDups(descs []PermissionDesc) bool {
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Less(descs[j])
	})
	for i := range descs {
		if i == 0 {
			continue
		}
		j := i - 1
		if descs[i].Type != descs[j].Type {
			continue
		}
		switch descs[i].Type {
		case PermissionWildcard:
			return true
		case PermissionHash:
			if descs[i].Hash() == descs[j].Hash() {
				return true
			}
		case PermissionGroup:
			if descs[i].Group().Cmp(descs[j].Group()) == 0 {
				return true
			}
		}
	}
	return false
}
