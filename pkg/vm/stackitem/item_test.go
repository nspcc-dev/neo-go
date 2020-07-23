package stackitem

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

var makeStackItemTestCases = []struct {
	input  interface{}
	result Item
}{
	{
		input:  int64(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  int16(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  3,
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  uint8(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  uint16(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  uint32(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  uint64(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  big.NewInt(3),
		result: &BigInteger{value: big.NewInt(3)},
	},
	{
		input:  []byte{1, 2, 3, 4},
		result: &ByteArray{value: []byte{1, 2, 3, 4}},
	},
	{
		input:  []byte{},
		result: &ByteArray{value: []byte{}},
	},
	{
		input:  "bla",
		result: &ByteArray{value: []byte("bla")},
	},
	{
		input:  "",
		result: &ByteArray{value: []byte{}},
	},
	{
		input:  true,
		result: &Bool{value: true},
	},
	{
		input:  false,
		result: &Bool{value: false},
	},
	{
		input:  []Item{&BigInteger{value: big.NewInt(3)}, &ByteArray{value: []byte{1, 2, 3}}},
		result: &Array{value: []Item{&BigInteger{value: big.NewInt(3)}, &ByteArray{value: []byte{1, 2, 3}}}},
	},
	{
		input:  []int{1, 2, 3},
		result: &Array{value: []Item{&BigInteger{value: big.NewInt(1)}, &BigInteger{value: big.NewInt(2)}, &BigInteger{value: big.NewInt(3)}}},
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
		assert.Equal(t, testCase.result, Make(testCase.input))
	}
	for _, errorCase := range makeStackItemErrorCases {
		assert.Panics(t, func() { Make(errorCase.input) })
	}
}

var stringerTestCases = []struct {
	input  Item
	result string
}{
	{
		input:  NewStruct([]Item{}),
		result: "Struct",
	},
	{
		input:  NewBigInteger(big.NewInt(3)),
		result: "BigInteger",
	},
	{
		input:  NewBool(true),
		result: "Boolean",
	},
	{
		input:  NewByteArray([]byte{}),
		result: "ByteArray",
	},
	{
		input:  NewArray([]Item{}),
		result: "Array",
	},
	{
		input:  NewMap(),
		result: "Map",
	},
	{
		input:  NewInterop(nil),
		result: "Interop",
	},
	{
		input:  NewPointer(0, nil),
		result: "Pointer",
	},
}

func TestStringer(t *testing.T) {
	for _, testCase := range stringerTestCases {
		assert.Equal(t, testCase.result, testCase.input.String())
	}
}

var equalsTestCases = map[string][]struct {
	item1  Item
	item2  Item
	result bool
}{
	"struct": {
		{
			item1:  NewStruct(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewStruct(nil),
			item2:  NewBigInteger(big.NewInt(1)),
			result: false,
		},
		{
			item1:  NewStruct(nil),
			item2:  NewStruct([]Item{NewBigInteger(big.NewInt(1))}),
			result: false,
		},
		{
			item1:  NewStruct([]Item{NewBigInteger(big.NewInt(1))}),
			item2:  NewStruct([]Item{NewBigInteger(big.NewInt(2))}),
			result: false,
		},
		{
			item1:  NewStruct([]Item{NewBigInteger(big.NewInt(1))}),
			item2:  NewStruct([]Item{NewBigInteger(big.NewInt(1))}),
			result: true,
		},
	},
	"bigint": {
		{
			item1:  NewBigInteger(big.NewInt(2)),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewBigInteger(big.NewInt(2)),
			item2:  NewBigInteger(big.NewInt(2)),
			result: true,
		},
		{
			item1:  NewBigInteger(big.NewInt(2)),
			item2:  NewBool(false),
			result: false,
		},
		{
			item1:  NewBigInteger(big.NewInt(0)),
			item2:  NewBool(false),
			result: false,
		},
		{
			item1:  NewBigInteger(big.NewInt(2)),
			item2:  Make(int32(2)),
			result: true,
		},
	},
	"bool": {
		{
			item1:  NewBool(true),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewBool(true),
			item2:  NewBool(true),
			result: true,
		},
		{
			item1:  NewBool(true),
			item2:  NewBigInteger(big.NewInt(1)),
			result: false,
		},
		{
			item1:  NewBool(true),
			item2:  NewBool(false),
			result: false,
		},
		{
			item1:  NewBool(true),
			item2:  Make(true),
			result: true,
		},
	},
	"bytearray": {
		{
			item1:  NewByteArray(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewByteArray([]byte{1, 2, 3}),
			item2:  NewByteArray([]byte{1, 2, 3}),
			result: true,
		},
		{
			item1:  NewByteArray([]byte{1}),
			item2:  NewBigInteger(big.NewInt(1)),
			result: false,
		},
		{
			item1:  NewByteArray([]byte{1, 2, 3}),
			item2:  NewByteArray([]byte{1, 2, 4}),
			result: false,
		},
		{
			item1:  NewByteArray([]byte{1, 2, 3}),
			item2:  Make([]byte{1, 2, 3}),
			result: true,
		},
	},
	"array": {
		{
			item1:  NewArray(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewArray([]Item{&BigInteger{big.NewInt(1)}, &BigInteger{big.NewInt(2)}, &BigInteger{big.NewInt(3)}}),
			item2:  NewArray([]Item{&BigInteger{big.NewInt(1)}, &BigInteger{big.NewInt(2)}, &BigInteger{big.NewInt(3)}}),
			result: false,
		},
		{
			item1:  NewArray([]Item{&BigInteger{big.NewInt(1)}}),
			item2:  NewBigInteger(big.NewInt(1)),
			result: false,
		},
		{
			item1:  NewArray([]Item{&BigInteger{big.NewInt(1)}, &BigInteger{big.NewInt(2)}, &BigInteger{big.NewInt(3)}}),
			item2:  NewArray([]Item{&BigInteger{big.NewInt(1)}, &BigInteger{big.NewInt(2)}, &BigInteger{big.NewInt(4)}}),
			result: false,
		},
	},
	"map": {
		{
			item1:  NewMap(),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewMap(),
			item2:  NewMap(),
			result: false,
		},
		{
			item1:  &Map{value: []MapElement{{NewByteArray([]byte("first")), NewBigInteger(big.NewInt(1))}, {NewBool(true), NewByteArray([]byte{2})}}},
			item2:  &Map{value: []MapElement{{NewByteArray([]byte("first")), NewBigInteger(big.NewInt(1))}, {NewBool(true), NewByteArray([]byte{2})}}},
			result: false,
		},
		{
			item1:  &Map{value: []MapElement{{NewByteArray([]byte("first")), NewBigInteger(big.NewInt(1))}, {NewBool(true), NewByteArray([]byte{2})}}},
			item2:  &Map{value: []MapElement{{NewByteArray([]byte("first")), NewBigInteger(big.NewInt(1))}, {NewBool(true), NewByteArray([]byte{3})}}},
			result: false,
		},
	},
	"interop": {
		{
			item1:  NewInterop(nil),
			item2:  nil,
			result: false,
		},
		{
			item1:  NewInterop(nil),
			item2:  NewInterop(nil),
			result: true,
		},
		{
			item1:  NewInterop(2),
			item2:  NewInterop(3),
			result: false,
		},
		{
			item1:  NewInterop(3),
			item2:  NewInterop(3),
			result: true,
		},
	},
	"pointer": {
		{
			item1:  NewPointer(0, []byte{}),
			result: false,
		},
		{
			item1:  NewPointer(1, []byte{1}),
			item2:  NewPointer(1, []byte{1}),
			result: true,
		},
		{
			item1:  NewPointer(1, []byte{1}),
			item2:  NewPointer(2, []byte{1}),
			result: false,
		},
		{
			item1:  NewPointer(1, []byte{1}),
			item2:  NewPointer(1, []byte{2}),
			result: false,
		},
		{
			item1:  NewPointer(0, []byte{}),
			item2:  NewBigInteger(big.NewInt(0)),
			result: false,
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
	input  Item
	result []byte
}{
	{
		input:  NewBigInteger(big.NewInt(2)),
		result: []byte(`2`),
	},
	{
		input:  NewBool(true),
		result: []byte(`true`),
	},
	{
		input:  NewByteArray([]byte{1, 2, 3}),
		result: []byte(`"010203"`),
	},
	{
		input:  NewBuffer([]byte{1, 2, 3}),
		result: []byte(`"010203"`),
	},
	{
		input:  &Array{value: []Item{&BigInteger{value: big.NewInt(3)}, &ByteArray{value: []byte{1, 2, 3}}}},
		result: []byte(`[3,"010203"]`),
	},
	{
		input:  &Interop{value: 3},
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
		case *BigInteger:
			actual, err = testCase.input.(*BigInteger).MarshalJSON()
		case *Bool:
			actual, err = testCase.input.(*Bool).MarshalJSON()
		case *ByteArray:
			actual, err = testCase.input.(*ByteArray).MarshalJSON()
		case *Array:
			actual, err = testCase.input.(*Array).MarshalJSON()
		case *Interop:
			actual, err = testCase.input.(*Interop).MarshalJSON()
		default:
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, testCase.result, actual)
	}
}
