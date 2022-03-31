package smartcontract

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var marshalJSONTestCases = []struct {
	input  Parameter
	result string
}{
	{
		input:  Parameter{Type: IntegerType, Value: big.NewInt(12345)},
		result: `{"type":"Integer","value":12345}`,
	},
	{
		input:  Parameter{Type: IntegerType, Value: new(big.Int).Lsh(big.NewInt(1), 254)},
		result: `{"type":"Integer","value":"` + new(big.Int).Lsh(big.NewInt(1), 254).String() + `"}`,
	},
	{
		input:  Parameter{Type: StringType, Value: "Some string"},
		result: `{"type":"String","value":"Some string"}`,
	},
	{
		input:  Parameter{Type: BoolType, Value: true},
		result: `{"type":"Boolean","value":true}`,
	},
	{
		input:  Parameter{Type: ByteArrayType, Value: []byte{0x01, 0x02, 0x03}},
		result: `{"type":"ByteString","value":"` + hexToBase64("010203") + `"}`,
	},
	{
		input:  Parameter{Type: ByteArrayType},
		result: `{"type":"ByteString","value":null}`,
	},
	{
		input:  Parameter{Type: SignatureType},
		result: `{"type":"Signature"}`,
	},
	{
		input: Parameter{
			Type:  PublicKeyType,
			Value: []byte{0x03, 0xb3, 0xbf, 0x15, 0x02, 0xfb, 0xdc, 0x05, 0x44, 0x9b, 0x50, 0x6a, 0xaf, 0x04, 0x57, 0x97, 0x24, 0x02, 0x4b, 0x06, 0x54, 0x2e, 0x49, 0x26, 0x2b, 0xfa, 0xa3, 0xf7, 0x0e, 0x20, 0x00, 0x40, 0xa9},
		},
		result: `{"type":"PublicKey","value":"03b3bf1502fbdc05449b506aaf04579724024b06542e49262bfaa3f70e200040a9"}`,
	},
	{
		input: Parameter{
			Type: ArrayType,
			Value: []Parameter{
				{Type: StringType, Value: "str 1"},
				{Type: IntegerType, Value: big.NewInt(2)},
			},
		},
		result: `{"type":"Array","value":[{"type":"String","value":"str 1"},{"type":"Integer","value":2}]}`,
	},
	{
		input: Parameter{
			Type: ArrayType,
			Value: []Parameter{
				{Type: ByteArrayType, Value: []byte{1, 2}},
				{
					Type: ArrayType,
					Value: []Parameter{
						{Type: ByteArrayType, Value: []byte{3, 2, 1}},
						{Type: ByteArrayType, Value: []byte{7, 8, 9}},
					}},
			},
		},
		result: `{"type":"Array","value":[{"type":"ByteString","value":"` + hexToBase64("0102") + `"},{"type":"Array","value":[` +
			`{"type":"ByteString","value":"` + hexToBase64("030201") + `"},{"type":"ByteString","value":"` + hexToBase64("070809") + `"}]}]}`,
	},
	{
		input: Parameter{
			Type: MapType,
			Value: []ParameterPair{
				{
					Key:   Parameter{Type: StringType, Value: "key1"},
					Value: Parameter{Type: IntegerType, Value: big.NewInt(1)},
				},
				{
					Key:   Parameter{Type: StringType, Value: "key2"},
					Value: Parameter{Type: StringType, Value: "two"},
				},
			},
		},
		result: `{"type":"Map","value":[{"key":{"type":"String","value":"key1"},"value":{"type":"Integer","value":1}},{"key":{"type":"String","value":"key2"},"value":{"type":"String","value":"two"}}]}`,
	},
	{
		input: Parameter{
			Type: MapType,
			Value: []ParameterPair{
				{
					Key: Parameter{Type: StringType, Value: "key1"},
					Value: Parameter{Type: ArrayType, Value: []Parameter{
						{Type: StringType, Value: "str 1"},
						{Type: IntegerType, Value: big.NewInt(2)},
					}},
				},
			},
		},
		result: `{"type":"Map","value":[{"key":{"type":"String","value":"key1"},"value":{"type":"Array","value":[{"type":"String","value":"str 1"},{"type":"Integer","value":2}]}}]}`,
	},
	{
		input: Parameter{
			Type: Hash160Type,
			Value: util.Uint160{
				0xd6, 0x24, 0x87, 0x12, 0xff, 0x97, 0x22, 0x80, 0xa0, 0xae,
				0xf5, 0x24, 0x1c, 0x96, 0x4d, 0x63, 0x78, 0x29, 0xcd, 0xb,
			},
		},
		result: `{"type":"Hash160","value":"0x0bcd2978634d961c24f5aea0802297ff128724d6"}`,
	},
	{
		input: Parameter{
			Type: Hash256Type,
			Value: util.Uint256{
				0x2d, 0xf3, 0x45, 0xf2, 0x45, 0xc5, 0x98, 0x9e,
				0x69, 0x95, 0x45, 0x06, 0xa5, 0x9e, 0x40, 0x12,
				0xc1, 0x68, 0x54, 0x48, 0x08, 0xfc, 0xcc, 0x5b,
				0x15, 0x18, 0xab, 0xa0, 0x8f, 0x30, 0x37, 0xf0,
			},
		},
		result: `{"type":"Hash256","value":"0xf037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"}`,
	},
	{
		input: Parameter{
			Type:  InteropInterfaceType,
			Value: nil,
		},
		result: `{"type":"InteropInterface","value":null}`,
	},
	{
		input: Parameter{
			Type:  ArrayType,
			Value: []Parameter{},
		},
		result: `{"type":"Array","value":[]}`,
	},
}

var marshalJSONErrorCases = []Parameter{
	{
		Type:  UnknownType,
		Value: nil,
	},
	{
		Type:  IntegerType,
		Value: math.Inf(1),
	},
}

func TestParam_MarshalJSON(t *testing.T) {
	for _, tc := range marshalJSONTestCases {
		res, err := json.Marshal(tc.input)
		assert.NoError(t, err)
		var actual, expected Parameter
		assert.NoError(t, json.Unmarshal(res, &actual))
		assert.NoError(t, json.Unmarshal([]byte(tc.result), &expected))

		assert.Equal(t, expected, actual)
	}

	for _, input := range marshalJSONErrorCases {
		_, err := json.Marshal(&input)
		assert.Error(t, err)
	}
}

var unmarshalJSONTestCases = []struct {
	input  string
	result Parameter
}{
	{
		input:  `{"type":"Bool","value":true}`,
		result: Parameter{Type: BoolType, Value: true},
	},
	{
		input:  `{"type":"Integer","value":12345}`,
		result: Parameter{Type: IntegerType, Value: big.NewInt(12345)},
	},
	{
		input:  `{"type":"Integer","value":"12345"}`,
		result: Parameter{Type: IntegerType, Value: big.NewInt(12345)},
	},
	{
		input:  `{"type":"ByteString","value":"` + hexToBase64("010203") + `"}`,
		result: Parameter{Type: ByteArrayType, Value: []byte{0x01, 0x02, 0x03}},
	},
	{
		input:  `{"type":"String","value":"Some string"}`,
		result: Parameter{Type: StringType, Value: "Some string"},
	},
	{
		input:  `{"type":"Signature"}`,
		result: Parameter{Type: SignatureType},
	},
	{
		input:  `{"type":"Signature","value":null }`,
		result: Parameter{Type: SignatureType},
	},
	{
		input: `{"type":"Array","value":[
				{"type": "String", "value": "str 1"},
				{"type": "Integer", "value": 2}]}`,
		result: Parameter{
			Type: ArrayType,
			Value: []Parameter{
				{Type: StringType, Value: "str 1"},
				{Type: IntegerType, Value: big.NewInt(2)},
			},
		},
	},
	{
		input: `{"type": "Hash160", "value": "0bcd2978634d961c24f5aea0802297ff128724d6"}`,
		result: Parameter{
			Type: Hash160Type,
			Value: util.Uint160{
				0xd6, 0x24, 0x87, 0x12, 0xff, 0x97, 0x22, 0x80, 0xa0, 0xae,
				0xf5, 0x24, 0x1c, 0x96, 0x4d, 0x63, 0x78, 0x29, 0xcd, 0xb,
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
	{
		input: `{"type":"Map","value":[{"key":{"type":"String","value":"key1"},"value":{"type":"Integer","value":1}},{"key":{"type":"String","value":"key2"},"value":{"type":"String","value":"two"}}]}`,
		result: Parameter{
			Type: MapType,
			Value: []ParameterPair{
				{
					Key:   Parameter{Type: StringType, Value: "key1"},
					Value: Parameter{Type: IntegerType, Value: big.NewInt(1)},
				},
				{
					Key:   Parameter{Type: StringType, Value: "key2"},
					Value: Parameter{Type: StringType, Value: "two"},
				},
			},
		},
	},
	{
		input: `{"type":"Map","value":[{"key":{"type":"String","value":"key1"},"value":{"type":"Array","value":[{"type":"String","value":"str 1"},{"type":"Integer","value":2}]}}]}`,
		result: Parameter{
			Type: MapType,
			Value: []ParameterPair{
				{
					Key: Parameter{Type: StringType, Value: "key1"},
					Value: Parameter{Type: ArrayType, Value: []Parameter{
						{Type: StringType, Value: "str 1"},
						{Type: IntegerType, Value: big.NewInt(2)},
					}},
				},
			},
		},
	},
	{
		result: Parameter{
			Type:  PublicKeyType,
			Value: []byte{0x03, 0xb3, 0xbf, 0x15, 0x02, 0xfb, 0xdc, 0x05, 0x44, 0x9b, 0x50, 0x6a, 0xaf, 0x04, 0x57, 0x97, 0x24, 0x02, 0x4b, 0x06, 0x54, 0x2e, 0x49, 0x26, 0x2b, 0xfa, 0xa3, 0xf7, 0x0e, 0x20, 0x00, 0x40, 0xa9},
		},
		input: `{"type":"PublicKey","value":"03b3bf1502fbdc05449b506aaf04579724024b06542e49262bfaa3f70e200040a9"}`,
	},
	{
		input: `{"type":"InteropInterface","value":null}`,
		result: Parameter{
			Type:  InteropInterfaceType,
			Value: nil,
		},
	},
	{
		input: `{"type":"InteropInterface","value":""}`,
		result: Parameter{
			Type:  InteropInterfaceType,
			Value: nil,
		},
	},
	{
		input: `{"type":"InteropInterface","value":"Hundertwasser"}`,
		result: Parameter{
			Type:  InteropInterfaceType,
			Value: nil,
		},
	},
}

var unmarshalJSONErrorCases = []string{
	`{"type": "ByteString","value":`,       // incorrect JSON
	`{"type": "ByteString","value":1}`,     // incorrect Value
	`{"type": "ByteString","value":"12^"}`, // incorrect ByteArray value
	`{"type": "String","value":`,           // incorrect JSON
	`{"type": "String","value":1}`,         // incorrect Value
	`{"type": "Integer","value": "nn"}`,    // incorrect Integer value
	`{"type": "Integer","value": []}`,      // incorrect Integer value
	`{"type": "Integer","value":"` +
		strings.Repeat("9", 100) + `"}`, // too big Integer
	`{"type": "Array","value": 123}`,       // incorrect Array value
	`{"type": "Hash160","value": "0bcd"}`,  // incorrect Uint160 value
	`{"type": "Hash256","value": "0bcd"}`,  // incorrect Uint256 value
	`{"type": "Stringg","value": ""}`,      // incorrect type
	`{"type": {},"value": ""}`,             // incorrect value
	`{"type": "Boolean","value": qwerty}`,  // incorrect Bool value
	`{"type": "Boolean","value": ""}`,      // incorrect Bool value
	`{"type": "Map","value": ["key": {}]}`, // incorrect Map value
	`{"type": "Map","value": ["key": {"type":"String", "value":"qwer"}, "value": {"type":"Boolean"}]}`, // incorrect Map Value value
	`{"type": "Map","value": ["key": {"type":"String"}, "value": {"type":"Boolean", "value":true}]}`,   // incorrect Map Key value
}

func TestParam_UnmarshalJSON(t *testing.T) {
	var s Parameter
	for _, tc := range unmarshalJSONTestCases {
		assert.NoError(t, json.Unmarshal([]byte(tc.input), &s))
		assert.Equal(t, s, tc.result)
	}

	for _, input := range unmarshalJSONErrorCases {
		assert.Error(t, json.Unmarshal([]byte(input), &s), input)
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
		input:    []byte{0x63, 0x78, 0x29, 0xcd, 0x0b},
		expected: big.NewInt(50686687331),
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
		out: Parameter{IntegerType, big.NewInt(42)},
	}, {
		in:  "Hello, 世界",
		out: Parameter{StringType, "Hello, 世界"},
	}, {
		in:  `\4\2`,
		out: Parameter{IntegerType, big.NewInt(42)},
	}, {
		in:  `\\4\2`,
		out: Parameter{StringType, `\42`},
	}, {
		in:  `\\\4\2`,
		out: Parameter{StringType, `\42`},
	}, {
		in:  "int:42",
		out: Parameter{IntegerType, big.NewInt(42)},
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
	}, {
		in:  "filebytes:./testdata/adjustValToType_filebytes_good.txt",
		out: Parameter{Type: ByteArrayType, Value: []byte{0x30, 0x31, 0x30, 0x32, 0x30, 0x33, 0x65, 0x66}},
	}, {
		in:  "filebytes:./testdata/does_not_exists.txt",
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

func hexToBase64(s string) string {
	b, _ := hex.DecodeString(s)
	return base64.StdEncoding.EncodeToString(b)
}

func TestExpandParameterToEmitable(t *testing.T) {
	pk, _ := keys.NewPrivateKey()
	testCases := []struct {
		In       Parameter
		Expected interface{}
	}{
		{
			In:       Parameter{Type: BoolType, Value: true},
			Expected: true,
		},
		{
			In:       Parameter{Type: IntegerType, Value: big.NewInt(123)},
			Expected: big.NewInt(123),
		},
		{
			In:       Parameter{Type: ByteArrayType, Value: []byte{1, 2, 3}},
			Expected: []byte{1, 2, 3},
		},
		{
			In:       Parameter{Type: StringType, Value: "writing's on the wall"},
			Expected: "writing's on the wall",
		},
		{
			In:       Parameter{Type: Hash160Type, Value: util.Uint160{1, 2, 3}},
			Expected: util.Uint160{1, 2, 3},
		},
		{
			In:       Parameter{Type: Hash256Type, Value: util.Uint256{1, 2, 3}},
			Expected: util.Uint256{1, 2, 3},
		},
		{
			In:       Parameter{Type: PublicKeyType, Value: pk.PublicKey()},
			Expected: pk.PublicKey().Bytes(),
		},
		{
			In:       Parameter{Type: SignatureType, Value: []byte{1, 2, 3}},
			Expected: []byte{1, 2, 3},
		},
		{
			In: Parameter{Type: ArrayType, Value: []Parameter{
				{
					Type:  IntegerType,
					Value: big.NewInt(123),
				},
				{
					Type:  ByteArrayType,
					Value: []byte{1, 2, 3},
				},
				{
					Type: ArrayType,
					Value: []Parameter{
						{
							Type:  BoolType,
							Value: true,
						},
					},
				},
			}},
			Expected: []interface{}{big.NewInt(123), []byte{1, 2, 3}, []interface{}{true}},
		},
	}
	bw := io.NewBufBinWriter()
	for _, testCase := range testCases {
		actual, err := ExpandParameterToEmitable(testCase.In)
		require.NoError(t, err)
		require.Equal(t, testCase.Expected, actual)

		emit.Array(bw.BinWriter, actual)
		require.NoError(t, bw.Err)
	}
	errCases := []Parameter{
		{Type: AnyType},
		{Type: UnknownType},
		{Type: MapType},
		{Type: InteropInterfaceType},
	}
	for _, errCase := range errCases {
		_, err := ExpandParameterToEmitable(errCase)
		require.Error(t, err)
	}
}
