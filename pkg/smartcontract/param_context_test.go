package smartcontract

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

var testCases = []struct {
	input  string
	result Parameter
}{
	{
		input:  `{"type":"Integer","value":12345}`,
		result: Parameter{Type: IntegerType, Value: int64(12345)},
	},
	{
		input:  `{"type":"Integer","value":"12345"}`,
		result: Parameter{Type: IntegerType, Value: int64(12345)},
	},
	{
		input:  `{"type":"ByteArray","value":"010203"}`,
		result: Parameter{Type: ByteArrayType, Value: []byte{0x01, 0x02, 0x03}},
	},
	{
		input:  `{"type":"String","value":"Some string"}`,
		result: Parameter{Type: StringType, Value: "Some string"},
	},
	{
		input: `{"type":"Array","value":[
				{"type": "String", "value": "str 1"},
				{"type": "Integer", "value": 2}]}`,
		result: Parameter{
			Type: ArrayType,
			Value: []Parameter{
				{Type: StringType, Value: "str 1"},
				{Type: IntegerType, Value: int64(2)},
			},
		},
	},
	{
		input: `{"type": "Hash160", "value": "0bcd2978634d961c24f5aea0802297ff128724d6"}`,
		result: Parameter{
			Type: Hash160Type,
			Value: util.Uint160{
				0x0b, 0xcd, 0x29, 0x78, 0x63, 0x4d, 0x96, 0x1c, 0x24, 0xf5,
				0xae, 0xa0, 0x80, 0x22, 0x97, 0xff, 0x12, 0x87, 0x24, 0xd6,
			},
		},
	},
	{
		input: `{"type": "Hash256", "value": "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"}`,
		result: Parameter{
			Type: Hash256Type,
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
	`{"type": "Map","value": ""}`,              //unmarshable type
}

func TestParam_UnmarshalJSON(t *testing.T) {
	var s Parameter
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

func TestParam_TryParse(t *testing.T) {
	for _, tc := range tryParseTestCases {
		t.Run(reflect.TypeOf(tc.expected).String(), func(t *testing.T) {
			input := Parameter{
				Type:  ByteArrayType,
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

		params := Params{
			{
				Type:  ByteArrayType,
				Value: exp1.BytesBE(),
			},
			{
				Type:  ByteArrayType,
				Value: exp2.BytesBE(),
			},
		}

		var out1, out2 util.Uint160

		assert.NoError(t, params.TryParseArray(&out1, &out2))
		assert.Equal(t, exp1, out1)
		assert.Equal(t, exp2, out2)
	})
}

func TestParamType_String(t *testing.T) {
	types := []ParamType{
		SignatureType,
		BoolType,
		IntegerType,
		Hash160Type,
		Hash256Type,
		ByteArrayType,
		PublicKeyType,
		StringType,
		ArrayType,
		InteropInterfaceType,
		MapType,
		VoidType,
	}

	for _, exp := range types {
		actual, err := ParseParamType(exp.String())
		assert.NoError(t, err)
		assert.Equal(t, exp, actual)
	}

	actual, err := ParseParamType(UnknownType.String())
	assert.Error(t, err)
	assert.Equal(t, UnknownType, actual)
}

func TestNewParameterFromString(t *testing.T) {
	var inouts = []struct {
		in  string
		out Parameter
		err bool
	}{{
		in:  "qwerty",
		out: Parameter{StringType, "qwerty"},
	}, {
		in:  "42",
		out: Parameter{IntegerType, 42},
	}, {
		in:  "Hello, 世界",
		out: Parameter{StringType, "Hello, 世界"},
	}, {
		in:  `\4\2`,
		out: Parameter{IntegerType, 42},
	}, {
		in:  `\\4\2`,
		out: Parameter{StringType, `\42`},
	}, {
		in:  `\\\4\2`,
		out: Parameter{StringType, `\42`},
	}, {
		in:  "int:42",
		out: Parameter{IntegerType, 42},
	}, {
		in:  "true",
		out: Parameter{BoolType, true},
	}, {
		in:  "string:true",
		out: Parameter{StringType, "true"},
	}, {
		in:  "\xfe\xff",
		err: true,
	}, {
		in:  `string\:true`,
		out: Parameter{StringType, "string:true"},
	}, {
		in:  "string:true:true",
		out: Parameter{StringType, "true:true"},
	}, {
		in:  `string\\:true`,
		err: true,
	}, {
		in:  `qwerty:asdf`,
		err: true,
	}, {
		in:  `bool:asdf`,
		err: true,
	}, {
		in:  `InteropInterface:123`,
		err: true,
	}, {
		in:  `Map:[]`,
		err: true,
	}}
	for _, inout := range inouts {
		out, err := NewParameterFromString(inout.in)
		if inout.err {
			assert.NotNil(t, err, "should error on '%s' input", inout.in)
		} else {
			assert.Nil(t, err, "shouldn't error on '%s' input", inout.in)
			assert.Equal(t, inout.out, *out, "bad output for '%s' input", inout.in)
		}
	}
}
