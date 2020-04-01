package vm

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/assert"
)

var makeStackItemTestCases = []struct {
	input  interface{}
	result StackItem
}{
	{
		input:  int64(3),
		result: &BigIntegerItem{value: big.NewInt(3)},
	},
	{
		input:  int16(3),
		result: &BigIntegerItem{value: big.NewInt(3)},
	},
	{
		input:  3,
		result: &BigIntegerItem{value: big.NewInt(3)},
	},
	{
		input:  uint8(3),
		result: &BigIntegerItem{value: big.NewInt(3)},
	},
	{
		input:  uint16(3),
		result: &BigIntegerItem{value: big.NewInt(3)},
	},
	{
		input:  uint32(3),
		result: &BigIntegerItem{value: big.NewInt(3)},
	},
	{
		input:  uint64(3),
		result: &BigIntegerItem{value: big.NewInt(3)},
	},
	{
		input:  big.NewInt(3),
		result: &BigIntegerItem{value: big.NewInt(3)},
	},
	{
		input:  []byte{1, 2, 3, 4},
		result: &ByteArrayItem{value: []byte{1, 2, 3, 4}},
	},
	{
		input:  []byte{},
		result: &ByteArrayItem{value: []byte{}},
	},
	{
		input:  "bla",
		result: &ByteArrayItem{value: []byte("bla")},
	},
	{
		input:  "",
		result: &ByteArrayItem{value: []byte{}},
	},
	{
		input:  true,
		result: &BoolItem{value: true},
	},
	{
		input:  false,
		result: &BoolItem{value: false},
	},
	{
		input:  []StackItem{&BigIntegerItem{value: big.NewInt(3)}, &ByteArrayItem{value: []byte{1, 2, 3}}},
		result: &ArrayItem{value: []StackItem{&BigIntegerItem{value: big.NewInt(3)}, &ByteArrayItem{value: []byte{1, 2, 3}}}},
	},
	{
		input:  []int{1, 2, 3},
		result: &ArrayItem{value: []StackItem{&BigIntegerItem{value: big.NewInt(1)}, &BigIntegerItem{value: big.NewInt(2)}, &BigIntegerItem{value: big.NewInt(3)}}},
	},
}

var makeStackItemErrorCases = []struct {
	input interface{}
}{
	{
		input: nil,
	},
}

func TestMakeStackItem(t *testing.T) {
	for _, testCase := range makeStackItemTestCases {
		assert.Equal(t, testCase.result, makeStackItem(testCase.input))
	}
	for _, errorCase := range makeStackItemErrorCases {
		assert.Panics(t, func() { makeStackItem(errorCase.input) })
	}
}

var stringerTestCases = []struct {
	input  StackItem
	result string
}{
	{
		input:  NewStructItem([]StackItem{}),
		result: "Struct",
	},
	{
		input:  NewBigIntegerItem(3),
		result: "BigInteger",
	},
	{
		input:  NewBoolItem(true),
		result: "Boolean",
	},
	{
		input:  NewByteArrayItem([]byte{}),
		result: "ByteArray",
	},
	{
		input:  NewArrayItem([]StackItem{}),
		result: "Array",
	},
	{
		input:  NewMapItem(),
		result: "Map",
	},
	{
		input:  NewInteropItem(nil),
		result: "InteropItem",
	},
}

func TestStringer(t *testing.T) {
	for _, testCase := range stringerTestCases {
		assert.Equal(t, testCase.result, testCase.input.String())
	}
}

var equalsTestCases = map[string][]struct {
	item1  StackItem
	item2  StackItem
	result bool
}{
	"struct": {
		{
			item1:  NewStructItem(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewStructItem(nil),
			item2:  NewBigIntegerItem(1),
			result: false,
		},
		{
			item1:  NewStructItem(nil),
			item2:  NewStructItem([]StackItem{NewBigIntegerItem(1)}),
			result: false,
		},
		{
			item1:  NewStructItem([]StackItem{NewBigIntegerItem(1)}),
			item2:  NewStructItem([]StackItem{NewBigIntegerItem(2)}),
			result: false,
		},
		{
			item1:  NewStructItem([]StackItem{NewBigIntegerItem(1)}),
			item2:  NewStructItem([]StackItem{NewBigIntegerItem(1)}),
			result: true,
		},
	},
	"bigint": {
		{
			item1:  NewBigIntegerItem(2),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewBigIntegerItem(2),
			item2:  NewBigIntegerItem(2),
			result: true,
		},
		{
			item1:  NewBigIntegerItem(2),
			item2:  NewBoolItem(false),
			result: false,
		},
		{
			item1:  NewBigIntegerItem(0),
			item2:  NewBoolItem(false),
			result: false,
		},
		{
			item1:  NewBigIntegerItem(2),
			item2:  makeStackItem(int32(2)),
			result: true,
		},
	},
	"bool": {
		{
			item1:  NewBoolItem(true),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewBoolItem(true),
			item2:  NewBoolItem(true),
			result: true,
		},
		{
			item1:  NewBoolItem(true),
			item2:  NewBigIntegerItem(1),
			result: true,
		},
		{
			item1:  NewBoolItem(true),
			item2:  NewBoolItem(false),
			result: false,
		},
		{
			item1:  NewBoolItem(true),
			item2:  makeStackItem(true),
			result: true,
		},
	},
	"bytearray": {
		{
			item1:  NewByteArrayItem(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewByteArrayItem([]byte{1, 2, 3}),
			item2:  NewByteArrayItem([]byte{1, 2, 3}),
			result: true,
		},
		{
			item1:  NewByteArrayItem([]byte{1}),
			item2:  NewBigIntegerItem(1),
			result: true,
		},
		{
			item1:  NewByteArrayItem([]byte{1, 2, 3}),
			item2:  NewByteArrayItem([]byte{1, 2, 4}),
			result: false,
		},
		{
			item1:  NewByteArrayItem([]byte{1, 2, 3}),
			item2:  makeStackItem([]byte{1, 2, 3}),
			result: true,
		},
	},
	"array": {
		{
			item1:  NewArrayItem(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewArrayItem([]StackItem{&BigIntegerItem{big.NewInt(1)}, &BigIntegerItem{big.NewInt(2)}, &BigIntegerItem{big.NewInt(3)}}),
			item2:  NewArrayItem([]StackItem{&BigIntegerItem{big.NewInt(1)}, &BigIntegerItem{big.NewInt(2)}, &BigIntegerItem{big.NewInt(3)}}),
			result: false,
		},
		{
			item1:  NewArrayItem([]StackItem{&BigIntegerItem{big.NewInt(1)}}),
			item2:  NewBigIntegerItem(1),
			result: false,
		},
		{
			item1:  NewArrayItem([]StackItem{&BigIntegerItem{big.NewInt(1)}, &BigIntegerItem{big.NewInt(2)}, &BigIntegerItem{big.NewInt(3)}}),
			item2:  NewArrayItem([]StackItem{&BigIntegerItem{big.NewInt(1)}, &BigIntegerItem{big.NewInt(2)}, &BigIntegerItem{big.NewInt(4)}}),
			result: false,
		},
	},
	"map": {
		{
			item1:  NewMapItem(),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewMapItem(),
			item2:  NewMapItem(),
			result: false,
		},
		{
			item1:  &MapItem{value: []MapElement{{NewByteArrayItem([]byte("first")), NewBigIntegerItem(1)}, {NewBoolItem(true), NewByteArrayItem([]byte{2})}}},
			item2:  &MapItem{value: []MapElement{{NewByteArrayItem([]byte("first")), NewBigIntegerItem(1)}, {NewBoolItem(true), NewByteArrayItem([]byte{2})}}},
			result: false,
		},
		{
			item1:  &MapItem{value: []MapElement{{NewByteArrayItem([]byte("first")), NewBigIntegerItem(1)}, {NewBoolItem(true), NewByteArrayItem([]byte{2})}}},
			item2:  &MapItem{value: []MapElement{{NewByteArrayItem([]byte("first")), NewBigIntegerItem(1)}, {NewBoolItem(true), NewByteArrayItem([]byte{3})}}},
			result: false,
		},
	},
	"interop": {
		{
			item1:  NewInteropItem(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewInteropItem(nil),
			item2:  NewInteropItem(nil),
			result: true,
		},
		{
			item1:  NewInteropItem(2),
			item2:  NewInteropItem(3),
			result: false,
		},
		{
			item1:  NewInteropItem(3),
			item2:  NewInteropItem(3),
			result: true,
		},
	},
}

func TestEquals(t *testing.T) {
	for name, testBatch := range equalsTestCases {
		for _, testCase := range testBatch {
			t.Run(name, func(t *testing.T) {
				assert.Equal(t, testCase.result, testCase.item1.Equals(testCase.item2))
				// Reference equals
				assert.Equal(t, true, testCase.item1.Equals(testCase.item1))
			})
		}
	}
}

var marshalJSONTestCases = []struct {
	input  StackItem
	result []byte
}{
	{
		input:  NewBigIntegerItem(2),
		result: []byte(`2`),
	},
	{
		input:  NewBoolItem(true),
		result: []byte(`true`),
	},
	{
		input:  NewByteArrayItem([]byte{1, 2, 3}),
		result: []byte(`"010203"`),
	},
	{
		input:  &ArrayItem{value: []StackItem{&BigIntegerItem{value: big.NewInt(3)}, &ByteArrayItem{value: []byte{1, 2, 3}}}},
		result: []byte(`[3,"010203"]`),
	},
	{
		input:  &InteropItem{value: 3},
		result: []byte(`3`),
	},
}

func TestMarshalJSON(t *testing.T) {
	var (
		actual []byte
		err    error
	)
	for _, testCase := range marshalJSONTestCases {
		switch testCase.input.(type) {
		case *BigIntegerItem:
			actual, err = testCase.input.(*BigIntegerItem).MarshalJSON()
		case *BoolItem:
			actual, err = testCase.input.(*BoolItem).MarshalJSON()
		case *ByteArrayItem:
			actual, err = testCase.input.(*ByteArrayItem).MarshalJSON()
		case *ArrayItem:
			actual, err = testCase.input.(*ArrayItem).MarshalJSON()
		case *InteropItem:
			actual, err = testCase.input.(*InteropItem).MarshalJSON()
		default:
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, testCase.result, actual)
	}
}

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
		input: &MapItem{value: []MapElement{
			{NewBigIntegerItem(1), NewBoolItem(true)},
			{NewByteArrayItem([]byte("qwerty")), NewBigIntegerItem(3)},
			{NewBoolItem(true), NewBoolItem(false)},
		}},
		result: smartcontract.Parameter{
			Type: smartcontract.MapType,
			Value: []smartcontract.ParameterPair{
				{
					Key:   smartcontract.Parameter{Type: smartcontract.IntegerType, Value: int64(1)},
					Value: smartcontract.Parameter{Type: smartcontract.BoolType, Value: true},
				}, {
					Key:   smartcontract.Parameter{Type: smartcontract.ByteArrayType, Value: []byte("qwerty")},
					Value: smartcontract.Parameter{Type: smartcontract.IntegerType, Value: int64(3)},
				}, {

					Key:   smartcontract.Parameter{Type: smartcontract.BoolType, Value: true},
					Value: smartcontract.Parameter{Type: smartcontract.BoolType, Value: false},
				},
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
