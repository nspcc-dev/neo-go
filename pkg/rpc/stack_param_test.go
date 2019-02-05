package rpc

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
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
}

func TestStackParam_UnmarshalJSON(t *testing.T) {
	var (
		err  error
		r, s StackParam
	)
	for _, tc := range testCases {
		if err = json.Unmarshal([]byte(tc.input), &s); err != nil {
			t.Errorf("error while unmarhsalling: %v", err)
		} else if !reflect.DeepEqual(s, tc.result) {
			t.Errorf("got (%v), expected (%v)", s, tc.result)
		}
	}

	// Hash160 unmarshalling
	err = json.Unmarshal([]byte(`{"type": "Hash160","value": "0bcd2978634d961c24f5aea0802297ff128724d6"}`), &s)
	if err != nil {
		t.Errorf("error while unmarhsalling: %v", err)
	}

	h160, err := util.Uint160DecodeString("0bcd2978634d961c24f5aea0802297ff128724d6")
	if err != nil {
		t.Errorf("unmarshal error: %v", err)
	}

	if r = (StackParam{Type: Hash160, Value: h160}); !reflect.DeepEqual(s, r) {
		t.Errorf("got (%v), expected (%v)", s, r)
	}

	// Hash256 unmarshalling
	err = json.Unmarshal([]byte(`{"type": "Hash256","value": "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"}`), &s)
	if err != nil {
		t.Errorf("error while unmarhsalling: %v", err)
	}
	h256, err := util.Uint256DecodeString("f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d")
	if err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if r = (StackParam{Type: Hash256, Value: h256}); !reflect.DeepEqual(s, r) {
		t.Errorf("got (%v), expected (%v)", s, r)
	}

	for _, input := range errorCases {
		if err = json.Unmarshal([]byte(input), &s); err == nil {
			t.Errorf("expected error, got (nil)")
		}
	}
}

const (
	hash160    = "0bcd2978634d961c24f5aea0802297ff128724d6"
	hash256    = "7fe610b7c8259ae949accacb091a1bc53219c51a1cb8752fbc6457674c13ec0b"
	testString = "myteststring"
)

func TestStackParam_TryParse(t *testing.T) {
	// ByteArray to util.Uint160 conversion
	data, err := hex.DecodeString(hash160)
	var (
		outputUint160, expectedUint160 util.Uint160
		input                          = StackParam{
			Type:  ByteArray,
			Value: data,
		}
	)
	expectedUint160, err = util.Uint160DecodeString(hash160)
	if err = input.TryParse(&outputUint160); err != nil {
		t.Errorf("failed to parse stackparam to Uint160: %v", err)
	}
	if !reflect.DeepEqual(outputUint160, expectedUint160) {
		t.Errorf("got (%v), expected (%v)", outputUint160, expectedUint160)
	}

	// ByteArray to util.Uint256 conversion
	data, err = hex.DecodeString(hash256)
	var (
		outputUint256, expectedUint256 util.Uint256
		uint256input                   = StackParam{
			Type:  ByteArray,
			Value: data,
		}
	)
	expectedUint256, err = util.Uint256DecodeString(hash256)
	if err = uint256input.TryParse(&outputUint256); err != nil {
		t.Errorf("failed to parse stackparam to []byte: %v", err)
	}
	if !reflect.DeepEqual(outputUint256, expectedUint256) {
		t.Errorf("got (%v), expected (%v)", outputUint256, expectedUint256)
	}

	// ByteArray to []byte conversion
	var (
		outputBytes   []byte
		expectedBytes = expectedUint160.Bytes()
	)
	if err = input.TryParse(&outputBytes); err != nil {
		t.Errorf("failed to parse stackparam to []byte: %v", err)
	}
	if !reflect.DeepEqual(outputBytes, expectedBytes) {
		t.Errorf("got (%v), expected (%v)", outputBytes, expectedBytes)
	}

	// ByteArray to int64 conversion
	data, err = hex.DecodeString("637829cd0b")
	var (
		outputInt, expectedInt int64
		intinput               = StackParam{
			Type:  ByteArray,
			Value: data,
		}
	)
	expectedInt = 50686687331
	if err = intinput.TryParse(&outputInt); err != nil {
		t.Errorf("failed to parse stackparam to []byte: %v", err)
	}
	if !reflect.DeepEqual(outputInt, expectedInt) {
		t.Errorf("got (%v), expected (%v)", outputInt, expectedInt)
	}

	// ByteArray to string conversion
	data = []byte(testString)
	var (
		outputStr, expectedStr string
		strinput               = StackParam{
			Type:  ByteArray,
			Value: data,
		}
	)
	expectedStr = testString
	if err = strinput.TryParse(&outputStr); err != nil {
		t.Errorf("failed to parse stackparam to []byte: %v", err)
	}
	if !reflect.DeepEqual(outputStr, expectedStr) {
		t.Errorf("got (%v), expected (%v)", outputStr, expectedStr)
	}

	// StackParams to []util.Uint160
	data, err = hex.DecodeString(hash160)
	expUint160, err := util.Uint160DecodeString(hash160)
	var (
		params = StackParams{
			StackParam{
				Type:  ByteArray,
				Value: data,
			},
			StackParam{
				Type:  ByteArray,
				Value: data,
			},
		}
		expectedArray = []util.Uint160{
			expUint160,
			expUint160,
		}
		out1, out2 = &util.Uint160{}, &util.Uint160{}
	)
	if err = params.TryParseArray(out1, out2); err != nil {
		t.Errorf("failed to parse stackparam to []byte: %v", err)
	}
	outArray := []util.Uint160{*out1, *out2}
	if !reflect.DeepEqual(outArray, expectedArray) {
		t.Errorf("got (%v), expected (%v)", outArray, expectedArray)
	}

}
