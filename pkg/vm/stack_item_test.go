package vm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CityOfZion/neo-go/pkg/smartcontract"
)

var toContractParameterTestCases = []struct {
	input  StackItem
	result smartcontract.Parameter
}{
	{
		input: NewStructItem([]StackItem{
			NewBigIntegerItem(1),
			NewBoolItem(true),
		}),
		result: smartcontract.Parameter{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
			{Type: smartcontract.IntegerType, Value: int64(1)},
			{Type: smartcontract.BoolType, Value: true},
		}},
	},
	{
		input:  NewBoolItem(false),
		result: smartcontract.Parameter{Type: smartcontract.BoolType, Value: false},
	},
	{
		input:  NewByteArrayItem([]byte{0x01, 0x02, 0x03}),
		result: smartcontract.Parameter{Type: smartcontract.ByteArrayType, Value: []byte{0x01, 0x02, 0x03}},
	},
	{
		input: NewArrayItem([]StackItem{NewBigIntegerItem(2), NewBoolItem(true)}),
		result: smartcontract.Parameter{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
			{Type: smartcontract.IntegerType, Value: int64(2)},
			{Type: smartcontract.BoolType, Value: true},
		}},
	},
	{
		input:  NewInteropItem(nil),
		result: smartcontract.Parameter{Type: smartcontract.InteropInterfaceType, Value: nil},
	},
	{
		input: &MapItem{value: map[interface{}]StackItem{
			toMapKey(NewBigIntegerItem(1)):               NewBoolItem(true),
			toMapKey(NewByteArrayItem([]byte("qwerty"))): NewBigIntegerItem(3),
			toMapKey(NewBoolItem(true)):                  NewBoolItem(false),
		}},
		result: smartcontract.Parameter{
			Type: smartcontract.MapType,
			Value: map[smartcontract.Parameter]smartcontract.Parameter{
				{Type: smartcontract.IntegerType, Value: int64(1)}:   {Type: smartcontract.BoolType, Value: true},
				{Type: smartcontract.ByteArrayType, Value: "qwerty"}: {Type: smartcontract.IntegerType, Value: int64(3)},
				{Type: smartcontract.BoolType, Value: true}:          {Type: smartcontract.BoolType, Value: false},
			},
		},
	},
}

func TestToContractParameter(t *testing.T) {
	for _, tc := range toContractParameterTestCases {
		seen := make(map[StackItem]bool)
		res := tc.input.ToContractParameter(seen)
		assert.Equal(t, res, tc.result)
	}
}

var fromMapKeyTestCases = []struct {
	input  interface{}
	result StackItem
}{
	{
		input:  true,
		result: NewBoolItem(true),
	},
	{
		input:  int64(4),
		result: NewBigIntegerItem(4),
	},
	{
		input:  "qwerty",
		result: NewByteArrayItem([]byte("qwerty")),
	},
}

func TestFromMapKey(t *testing.T) {
	for _, tc := range fromMapKeyTestCases {
		res := fromMapKey(tc.input)
		assert.Equal(t, res, tc.result)
	}
}
