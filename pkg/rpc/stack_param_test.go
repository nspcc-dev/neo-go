package rpc

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

var testCases = []struct {
	input  string
	result StackParam
}{
	{
		input:  `{"type":"Integer","value":12345}`,
		result: StackParam{Type: Integer, Value: int64(12345)},
	},
	{
		input:  `{"type":"Integer","value":"12345"}`,
		result: StackParam{Type: Integer, Value: int64(12345)},
	},
	{
		input:  `{"type":"ByteArray","value":"010203"}`,
		result: StackParam{Type: ByteArray, Value: []byte{0x01, 0x02, 0x03}},
	},
	{
		input:  `{"type":"String","value":"Some string"}`,
		result: StackParam{Type: String, Value: "Some string"},
	},
	{
		input: `{"type":"Array","value":[
{"type": "String", "value": "str 1"},
{"type": "Integer", "value": 2}]}`,
		result: StackParam{
			Type: Array,
			Value: []StackParam{
				{Type: String, Value: "str 1"},
				{Type: Integer, Value: int64(2)},
			},
		},
	},
	{
		input: `{"type": "Hash160", "value": "0bcd2978634d961c24f5aea0802297ff128724d6"}`,
		result: StackParam{
			Type: Hash160,
			Value: util.Uint160{
				0x0b, 0xcd, 0x29, 0x78, 0x63, 0x4d, 0x96, 0x1c, 0x24, 0xf5,
				0xae, 0xa0, 0x80, 0x22, 0x97, 0xff, 0x12, 0x87, 0x24, 0xd6,
			},
		},
	},
	{
		input: `{"type": "Hash256", "value": "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"}`,
		result: StackParam{
			Type: Hash256,
			Value: util.Uint256{
				0x2d, 0xf3, 0x45, 0xf2, 0x45, 0xc5, 0x98, 0x9e,
				0x69, 0x95, 0x45, 0x06, 0xa5, 0x9e, 0x40, 0x12,
				0xc1, 0x68, 0x54, 0x48, 0x08, 0xfc, 0xcc, 0x5b,
				0x15, 0x18, 0xab, 0xa0, 0x8f, 0x30, 0x37, 0xf0,
			},
		},
	},
}

var errorCases = []string{
	`{"type": "ByteArray","value":`,        // incorrect JSON
	`{"type": "ByteArray","value":1}`,      // incorrect Value
	`{"type": "ByteArray","value":"12zz"}`, // incorrect ByteArray value
	`{"type": "String","value":`,           // incorrect JSON
	`{"type": "String","value":1}`,         // incorrect Value
	`{"type": "Integer","value": "nn"}`,    // incorrect Integer value
	`{"type": "Integer","value": []}`,      // incorrect Integer value
	`{"type": "Array","value": 123}`,       // incorrect Array value
	`{"type": "Hash160","value": "0bcd"}`,  // incorrect Uint160 value
	`{"type": "Hash256","value": "0bcd"}`,  // incorrect Uint256 value
	`{"type": "Stringg","value": ""}`,      // incorrect type
	`{"type": {},"value": ""}`,             // incorrect value

	`{"type": "InteropInterface","value": ""}`, // ununmarshable type
}

func TestStackParam_UnmarshalJSON(t *testing.T) {
	var s StackParam
	for _, tc := range testCases {
		assert.NoError(t, json.Unmarshal([]byte(tc.input), &s))
		assert.Equal(t, s, tc.result)
	}

	for _, input := range errorCases {
		assert.Error(t, json.Unmarshal([]byte(input), &s))
	}
}

var tryParseTestCases = []struct {
	input    interface{}
	expected interface{}
}{
	{
		input: []byte{
			0x0b, 0xcd, 0x29, 0x78, 0x63, 0x4d, 0x96, 0x1c, 0x24, 0xf5,
			0xae, 0xa0, 0x80, 0x22, 0x97, 0xff, 0x12, 0x87, 0x24, 0xd6,
		},
		expected: util.Uint160{
			0x0b, 0xcd, 0x29, 0x78, 0x63, 0x4d, 0x96, 0x1c, 0x24, 0xf5,
			0xae, 0xa0, 0x80, 0x22, 0x97, 0xff, 0x12, 0x87, 0x24, 0xd6,
		},
	},
	{
		input: []byte{
			0xf0, 0x37, 0x30, 0x8f, 0xa0, 0xab, 0x18, 0x15,
			0x5b, 0xcc, 0xfc, 0x08, 0x48, 0x54, 0x68, 0xc1,
			0x12, 0x40, 0x9e, 0xa5, 0x06, 0x45, 0x95, 0x69,
			0x9e, 0x98, 0xc5, 0x45, 0xf2, 0x45, 0xf3, 0x2d,
		},
		expected: util.Uint256{
			0x2d, 0xf3, 0x45, 0xf2, 0x45, 0xc5, 0x98, 0x9e,
			0x69, 0x95, 0x45, 0x06, 0xa5, 0x9e, 0x40, 0x12,
			0xc1, 0x68, 0x54, 0x48, 0x08, 0xfc, 0xcc, 0x5b,
			0x15, 0x18, 0xab, 0xa0, 0x8f, 0x30, 0x37, 0xf0,
		},
	},
	{
		input:    []byte{0, 1, 2, 3, 4, 9, 8, 6},
		expected: []byte{0, 1, 2, 3, 4, 9, 8, 6},
	},
	{
		input:    []byte{0x63, 0x78, 0x29, 0xcd, 0x0b},
		expected: int64(50686687331),
	},
	{
		input:    []byte("this is a test string"),
		expected: "this is a test string",
	},
}

func TestStackParam_TryParse(t *testing.T) {
	for _, tc := range tryParseTestCases {
		t.Run(reflect.TypeOf(tc.expected).String(), func(t *testing.T) {
			input := StackParam{
				Type:  ByteArray,
				Value: tc.input,
			}

			val := reflect.New(reflect.TypeOf(tc.expected))
			assert.NoError(t, input.TryParse(val.Interface()))
			assert.Equal(t, tc.expected, val.Elem().Interface())
		})
	}

	t.Run("[]Uint160", func(t *testing.T) {
		exp1 := util.Uint160{1, 2, 3, 4, 5}
		exp2 := util.Uint160{9, 8, 7, 6, 5}

		params := StackParams{
			{
				Type:  ByteArray,
				Value: exp1.Bytes(),
			},
			{
				Type:  ByteArray,
				Value: exp2.Bytes(),
			},
		}

		var out1, out2 util.Uint160

		assert.NoError(t, params.TryParseArray(&out1, &out2))
		assert.Equal(t, exp1, out1)
		assert.Equal(t, exp2, out2)
	})
}

func TestStackParamType_String(t *testing.T) {
	types := []StackParamType{
		Signature,
		Boolean,
		Integer,
		Hash160,
		Hash256,
		ByteArray,
		PublicKey,
		String,
		Array,
		InteropInterface,
		Void,
	}

	for _, exp := range types {
		actual, err := StackParamTypeFromString(exp.String())
		assert.NoError(t, err)
		assert.Equal(t, exp, actual)
	}

	actual, err := StackParamTypeFromString(Unknown.String())
	assert.Error(t, err)
	assert.Equal(t, Unknown, actual)
}
