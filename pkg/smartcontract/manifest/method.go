package manifest

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Method represents method's metadata.
type Method struct {
	Name       string                  `json:"name"`
	Offset     int                     `json:"offset"`
	Parameters []Parameter             `json:"parameters"`
	ReturnType smartcontract.ParamType `json:"returntype"`
	Safe       bool                    `json:"safe"`
}

// Ensure required interface are implemented for proper RPC bindings generation.
var _ = smartcontract.Convertible(&Method{})

// IsValid checks Method consistency and correctness.
func (m *Method) IsValid() error {
	if m.Name == "" {
		return errors.New("empty or absent name")
	}
	if m.Offset < 0 {
		return errors.New("negative offset")
	}
	_, err := smartcontract.ConvertToParamType(int(m.ReturnType))
	if err != nil {
		return err
	}
	return Parameters(m.Parameters).AreValid()
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

// ToSCParameter creates [smartcontract.Parameter] representing [Method]. It
// never returns an error. It implements [smartcontract.Convertible]
// interface.
func (m *Method) ToSCParameter() (smartcontract.Parameter, error) {
	params := make([]smartcontract.Parameter, len(m.Parameters))
	for i := range m.Parameters {
		prm, err := m.Parameters[i].ToSCParameter()
		if err != nil {
			return smartcontract.Parameter{}, err
		}
		params[i] = prm
	}
	return smartcontract.Parameter{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
		{Type: smartcontract.StringType, Value: m.Name},
		{Type: smartcontract.ArrayType, Value: params},
		{Type: smartcontract.IntegerType, Value: big.NewInt(int64(m.ReturnType))},
		{Type: smartcontract.IntegerType, Value: big.NewInt(int64(m.Offset))},
		{Type: smartcontract.BoolType, Value: m.Safe},
	}}, nil
}
