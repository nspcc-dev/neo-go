package manifest

import (
	"errors"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Event is a description of a single event.
type Event struct {
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters"`
}

// IsValid checks Event consistency and correctness.
func (e *Event) IsValid() error {
	if e.Name == "" {
		return errors.New("empty or absent name")
	}
	if len(e.Parameters) > 1 {
		paramNames := make([]string, len(e.Parameters))
		for i := range e.Parameters {
			paramNames[i] = e.Parameters[i].Name
		}
		sort.Strings(paramNames)
		for i := range paramNames {
			if i == 0 {
				continue
			}
			if paramNames[i] == paramNames[i-1] {
				return errors.New("duplicate parameter name")
			}
		}
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
