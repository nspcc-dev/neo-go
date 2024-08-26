package manifest

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MethodInit is a name for default initialization method.
	MethodInit = "_initialize"

	// MethodDeploy is a name for default method called during contract deployment.
	MethodDeploy = "_deploy"

	// MethodVerify is a name for default verification method.
	MethodVerify = "verify"

	// MethodOnNEP17Payment is the name of the method which is called when contract receives NEP-17 tokens.
	MethodOnNEP17Payment = "onNEP17Payment"

	// MethodOnNEP11Payment is the name of the method which is called when contract receives NEP-11 tokens.
	MethodOnNEP11Payment = "onNEP11Payment"
)

// ABI represents a contract application binary interface.
type ABI struct {
	Methods []Method `json:"methods"`
	Events  []Event  `json:"events"`
}

// GetMethod returns methods with the specified name.
func (a *ABI) GetMethod(name string, paramCount int) *Method {
	for i := range a.Methods {
		if a.Methods[i].Name == name && (paramCount == -1 || len(a.Methods[i].Parameters) == paramCount) {
			return &a.Methods[i]
		}
	}
	return nil
}

// GetEvent returns the event with the specified name.
func (a *ABI) GetEvent(name string) *Event {
	for i := range a.Events {
		if a.Events[i].Name == name {
			return &a.Events[i]
		}
	}
	return nil
}

// IsValid checks ABI consistency and correctness.
func (a *ABI) IsValid() error {
	if len(a.Methods) == 0 {
		return errors.New("no methods")
	}
	for i := range a.Methods {
		err := a.Methods[i].IsValid()
		if err != nil {
			return fmt.Errorf("method %q/%d: %w", a.Methods[i].Name, len(a.Methods[i].Parameters), err)
		}
	}
	if len(a.Methods) > 1 {
		var methods = slices.Clone(a.Methods)
		slices.SortFunc(methods, func(a, b Method) int {
			return cmp.Or(
				cmp.Compare(a.Name, b.Name),
				cmp.Compare(len(a.Parameters), len(b.Parameters)),
			)
		})
		for i := range methods {
			if i == 0 {
				continue
			}
			if methods[i].Name == methods[i-1].Name &&
				len(methods[i].Parameters) == len(methods[i-1].Parameters) {
				return errors.New("duplicate method specifications")
			}
		}
	}
	for i := range a.Events {
		err := a.Events[i].IsValid()
		if err != nil {
			return fmt.Errorf("event %q/%d: %w", a.Events[i].Name, len(a.Events[i].Parameters), err)
		}
	}
	if len(a.Events) > 1 {
		names := make([]string, len(a.Events))
		for i := range a.Events {
			names[i] = a.Events[i].Name
		}
		if stringsHaveDups(names) {
			return errors.New("duplicate event names")
		}
	}
	return nil
}

// ToStackItem converts ABI to stackitem.Item.
func (a *ABI) ToStackItem() stackitem.Item {
	methods := make([]stackitem.Item, len(a.Methods))
	for i := range a.Methods {
		methods[i] = a.Methods[i].ToStackItem()
	}
	events := make([]stackitem.Item, len(a.Events))
	for i := range a.Events {
		events[i] = a.Events[i].ToStackItem()
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(methods),
		stackitem.Make(events),
	})
}

// FromStackItem converts stackitem.Item to ABI.
func (a *ABI) FromStackItem(item stackitem.Item) error {
	if item.Type() != stackitem.StructT {
		return errors.New("invalid ABI stackitem type")
	}
	str := item.Value().([]stackitem.Item)
	if len(str) != 2 {
		return errors.New("invalid ABI stackitem length")
	}
	if str[0].Type() != stackitem.ArrayT {
		return errors.New("invalid Methods stackitem type")
	}
	methods := str[0].Value().([]stackitem.Item)
	a.Methods = make([]Method, len(methods))
	for i := range methods {
		m := new(Method)
		if err := m.FromStackItem(methods[i]); err != nil {
			return err
		}
		a.Methods[i] = *m
	}
	if str[1].Type() != stackitem.ArrayT {
		return errors.New("invalid Events stackitem type")
	}
	events := str[1].Value().([]stackitem.Item)
	a.Events = make([]Event, len(events))
	for i := range events {
		e := new(Event)
		if err := e.FromStackItem(events[i]); err != nil {
			return err
		}
		a.Events[i] = *e
	}
	return nil
}
