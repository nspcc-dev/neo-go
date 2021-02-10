package manifest

import (
	"errors"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MethodInit is a name for default initialization method.
	MethodInit = "_initialize"

	// MethodDeploy is a name for default method called during contract deployment.
	MethodDeploy = "_deploy"

	// MethodVerify is a name for default verification method.
	MethodVerify = "verify"

	// MethodOnNEP17Payment is name of the method which is called when contract receives NEP-17 tokens.
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

// GetEvent returns event with the specified name.
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
		return errors.New("ABI contains no methods")
	}
	for i := range a.Methods {
		err := a.Methods[i].IsValid()
		if err != nil {
			return err
		}
	}
	if len(a.Methods) > 1 {
		methods := make([]struct {
			name   string
			params int
		}, len(a.Methods))
		for i := range methods {
			methods[i].name = a.Methods[i].Name
			methods[i].params = len(a.Methods[i].Parameters)
		}
		sort.Slice(methods, func(i, j int) bool {
			if methods[i].name < methods[j].name {
				return true
			}
			if methods[i].name == methods[j].name {
				return methods[i].params < methods[j].params
			}
			return false
		})
		for i := range methods {
			if i == 0 {
				continue
			}
			if methods[i].name == methods[i-1].name &&
				methods[i].params == methods[i-1].params {
				return errors.New("duplicate method specifications")
			}
		}
	}
	for i := range a.Events {
		err := a.Events[i].IsValid()
		if err != nil {
			return err
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
