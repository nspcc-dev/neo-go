package manifest

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Event is a description of a single event.
type Event struct {
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters"`
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
