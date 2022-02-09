package smartcontract

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
)

var toContractParameterTestCases = []struct {
	input  stackitem.Item
	result Parameter
}{
	{
		input: stackitem.NewStruct([]stackitem.Item{
			stackitem.NewBigInteger(big.NewInt(1)),
			stackitem.NewBool(true),
		}),
		result: Parameter{Type: ArrayType, Value: []Parameter{
			{Type: IntegerType, Value: big.NewInt(1)},
			{Type: BoolType, Value: true},
		}},
	},
	{
		input:  stackitem.NewBool(false),
		result: Parameter{Type: BoolType, Value: false},
	},
	{
		input:  stackitem.NewByteArray([]byte{0x01, 0x02, 0x03}),
		result: Parameter{Type: ByteArrayType, Value: []byte{0x01, 0x02, 0x03}},
	},
	{
		input:  stackitem.NewBuffer([]byte{0x01, 0x02, 0x03}),
		result: Parameter{Type: ByteArrayType, Value: []byte{0x01, 0x02, 0x03}},
	},
	{
		input: stackitem.NewArray([]stackitem.Item{stackitem.NewBigInteger(big.NewInt(2)), stackitem.NewBool(true)}),
		result: Parameter{Type: ArrayType, Value: []Parameter{
			{Type: IntegerType, Value: big.NewInt(2)},
			{Type: BoolType, Value: true},
		}},
	},
	{
		input:  stackitem.NewInterop(nil),
		result: Parameter{Type: InteropInterfaceType, Value: nil},
	},
	{
		input: stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.NewBigInteger(big.NewInt(1)), Value: stackitem.NewBool(true)},
			{Key: stackitem.NewByteArray([]byte("qwerty")), Value: stackitem.NewBigInteger(big.NewInt(3))},
			{Key: stackitem.NewBool(true), Value: stackitem.NewBool(false)},
		}),
		result: Parameter{
			Type: MapType,
			Value: []ParameterPair{
				{
					Key:   Parameter{Type: IntegerType, Value: big.NewInt(1)},
					Value: Parameter{Type: BoolType, Value: true},
				}, {
					Key:   Parameter{Type: ByteArrayType, Value: []byte("qwerty")},
					Value: Parameter{Type: IntegerType, Value: big.NewInt(3)},
				}, {

					Key:   Parameter{Type: BoolType, Value: true},
					Value: Parameter{Type: BoolType, Value: false},
				},
			},
		},
	},
}

func TestToContractParameter(t *testing.T) {
	for _, tc := range toContractParameterTestCases {
		seen := make(map[stackitem.Item]bool)
		res := ParameterFromStackItem(tc.input, seen)
		assert.Equal(t, res, tc.result)
	}
}
