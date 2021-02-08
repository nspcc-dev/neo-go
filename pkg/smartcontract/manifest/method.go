package manifest

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Parameter represents smartcontract's parameter's definition.
type Parameter struct {
	Name string                  `json:"name"`
	Type smartcontract.ParamType `json:"type"`
}

// Event is a description of a single event.
type Event struct {
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters"`
}

// Method represents method's metadata.
type Method struct {
	Name       string                  `json:"name"`
	Offset     int                     `json:"offset"`
	Parameters []Parameter             `json:"parameters"`
	ReturnType smartcontract.ParamType `json:"returntype"`
	Safe       bool                    `json:"safe"`
}

// NewParameter returns new parameter of specified name and type.
func NewParameter(name string, typ smartcontract.ParamType) Parameter {
	return Parameter{
		Name: name,
		Type: typ,
	}
}

// ToStackItem converts Method to stackitem.Item.
func (m *Method) ToStackItem() stackitem.Item {
	params := make([]stackitem.Item, len(m.Parameters))
	for i := range m.Parameters {
		params[i] = m.Parameters[i].ToStackItem()
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(m.Name),
		stackitem.Make(params),
		stackitem.Make(int(m.ReturnType)),
		stackitem.Make(m.Offset),
		stackitem.Make(m.Safe),
	})
}

// FromStackItem converts stackitem.Item to Method.
func (m *Method) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Method stackitem type")
	}
	method := item.Value().([]stackitem.Item)
	if len(method) != 5 {
		return errors.New("invalid Method stackitem length")
	}
	m.Name, err = stackitem.ToString(method[0])
	if err != nil {
		return err
	}
	if method[1].Type() != stackitem.ArrayT {
		return errors.New("invalid Params stackitem type")
	}
	params := method[1].Value().([]stackitem.Item)
	m.Parameters = make([]Parameter, len(params))
	for i := range params {
		p := new(Parameter)
		if err := p.FromStackItem(params[i]); err != nil {
			return err
		}
		m.Parameters[i] = *p
	}
	rTyp, err := method[2].TryInteger()
	if err != nil {
		return err
	}
	m.ReturnType, err = smartcontract.ConvertToParamType(int(rTyp.Int64()))
	if err != nil {
		return err
	}
	offset, err := method[3].TryInteger()
	if err != nil {
		return err
	}
	m.Offset = int(offset.Int64())
	safe, err := method[4].TryBool()
	if err != nil {
		return err
	}
	m.Safe = safe
	return nil
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

// ToStackItem converts Event to stackitem.Item.
func (e *Event) ToStackItem() stackitem.Item {
	params := make([]stackitem.Item, len(e.Parameters))
	for i := range e.Parameters {
		params[i] = e.Parameters[i].ToStackItem()
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(e.Name),
		stackitem.Make(params),
	})
}

// FromStackItem converts stackitem.Item to Event.
func (e *Event) FromStackItem(item stackitem.Item) error {
	var err error
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Event stackitem type")
	}
	event := item.Value().([]stackitem.Item)
	if len(event) != 2 {
		return errors.New("invalid Event stackitem length")
	}
	e.Name, err = stackitem.ToString(event[0])
	if err != nil {
		return err
	}
	if event[1].Type() != stackitem.ArrayT {
		return errors.New("invalid Params stackitem type")
	}
	params := event[1].Value().([]stackitem.Item)
	e.Parameters = make([]Parameter, len(params))
	for i := range params {
		p := new(Parameter)
		if err := p.FromStackItem(params[i]); err != nil {
			return err
		}
		e.Parameters[i] = *p
	}
	return nil
}
