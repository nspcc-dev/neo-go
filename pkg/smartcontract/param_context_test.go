package smartcontract

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseParamType(t *testing.T) {
	var inouts = []struct {
		in  string
		out ParamType
		err bool
	}{{
		in:  "signature",
		out: SignatureType,
	}, {
		in:  "Signature",
		out: SignatureType,
	}, {
		in:  "SiGnAtUrE",
		out: SignatureType,
	}, {
		in:  "bool",
		out: BoolType,
	}, {
		in:  "int",
		out: IntegerType,
	}, {
		in:  "hash160",
		out: Hash160Type,
	}, {
		in:  "hash256",
		out: Hash256Type,
	}, {
		in:  "bytes",
		out: ByteArrayType,
	}, {
		in:  "key",
		out: PublicKeyType,
	}, {
		in:  "string",
		out: StringType,
	}, {
		in:  "array",
		err: true,
	}, {
		in:  "qwerty",
		err: true,
	}}
	for _, inout := range inouts {
		out, err := parseParamType(inout.in)
		if inout.err {
			assert.NotNil(t, err, "should error on '%s' input", inout.in)
		} else {
			assert.Nil(t, err, "shouldn't error on '%s' input", inout.in)
			assert.Equal(t, inout.out, out, "bad output for '%s' input", inout.in)
		}
	}
}

func TestInferParamType(t *testing.T) {
	var inouts = []struct {
		in  string
		out ParamType
	}{{
		in:  "42",
		out: IntegerType,
	}, {
		in:  "-42",
		out: IntegerType,
	}, {
		in:  "0",
		out: IntegerType,
	}, {
		in:  "2e10",
		out: ByteArrayType,
	}, {
		in:  "true",
		out: BoolType,
	}, {
		in:  "false",
		out: BoolType,
	}, {
		in:  "truee",
		out: StringType,
	}, {
		in:  "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y",
		out: Hash160Type,
	}, {
		in:  "ZK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y",
		out: StringType,
	}, {
		in:  "50befd26fdf6e4d957c11e078b24ebce6291456f",
		out: Hash160Type,
	}, {
		in:  "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c",
		out: PublicKeyType,
	}, {
		in:  "30b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c",
		out: ByteArrayType,
	}, {
		in:  "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
		out: Hash256Type,
	}, {
		in:  "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7da",
		out: ByteArrayType,
	}, {
		in:  "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
		out: SignatureType,
	}, {
		in:  "qwerty",
		out: StringType,
	}, {
		in:  "ab",
		out: ByteArrayType,
	}, {
		in:  "az",
		out: StringType,
	}, {
		in:  "bad",
		out: StringType,
	}, {
		in:  "фыва",
		out: StringType,
	}, {
		in:  "dead",
		out: ByteArrayType,
	}}
	for _, inout := range inouts {
		out := inferParamType(inout.in)
		assert.Equal(t, inout.out, out, "bad output for '%s' input", inout.in)
	}
}

func TestAdjustValToType(t *testing.T) {
	var inouts = []struct {
		typ ParamType
		val string
		out interface{}
		err bool
	}{{
		typ: SignatureType,
		val: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
		out: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
	}, {
		typ: SignatureType,
		val: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c",
		err: true,
	}, {
		typ: SignatureType,
		val: "qwerty",
		err: true,
	}, {
		typ: BoolType,
		val: "false",
		out: false,
	}, {
		typ: BoolType,
		val: "true",
		out: true,
	}, {
		typ: BoolType,
		val: "qwerty",
		err: true,
	}, {
		typ: BoolType,
		val: "42",
		err: true,
	}, {
		typ: BoolType,
		val: "0",
		err: true,
	}, {
		typ: IntegerType,
		val: "0",
		out: 0,
	}, {
		typ: IntegerType,
		val: "42",
		out: 42,
	}, {
		typ: IntegerType,
		val: "-42",
		out: -42,
	}, {
		typ: IntegerType,
		val: "q",
		err: true,
	}, {
		typ: Hash160Type,
		val: "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y",
		out: "23ba2703c53263e8d6e522dc32203339dcd8eee9",
	}, {
		typ: Hash160Type,
		val: "50befd26fdf6e4d957c11e078b24ebce6291456f",
		out: "50befd26fdf6e4d957c11e078b24ebce6291456f",
	}, {
		typ: Hash160Type,
		val: "befd26fdf6e4d957c11e078b24ebce6291456f",
		err: true,
	}, {
		typ: Hash160Type,
		val: "q",
		err: true,
	}, {
		typ: Hash256Type,
		val: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
		out: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
	}, {
		typ: Hash256Type,
		val: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282d",
		err: true,
	}, {
		typ: Hash256Type,
		val: "q",
		err: true,
	}, {
		typ: ByteArrayType,
		val: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282d",
		out: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282d",
	}, {
		typ: ByteArrayType,
		val: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
		out: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
	}, {
		typ: ByteArrayType,
		val: "50befd26fdf6e4d957c11e078b24ebce6291456f",
		out: "50befd26fdf6e4d957c11e078b24ebce6291456f",
	}, {
		typ: ByteArrayType,
		val: "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y",
		err: true,
	}, {
		typ: ByteArrayType,
		val: "q",
		err: true,
	}, {
		typ: ByteArrayType,
		val: "ab",
		out: "ab",
	}, {
		typ: PublicKeyType,
		val: "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c",
		out: "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c",
	}, {
		typ: PublicKeyType,
		val: "01b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c",
		err: true,
	}, {
		typ: PublicKeyType,
		val: "q",
		err: true,
	}, {
		typ: StringType,
		val: "q",
		out: "q",
	}, {
		typ: StringType,
		val: "dead",
		out: "dead",
	}, {
		typ: StringType,
		val: "йцукен",
		out: "йцукен",
	}, {
		typ: ArrayType,
		val: "",
		err: true,
	}}

	for _, inout := range inouts {
		out, err := adjustValToType(inout.typ, inout.val)
		if inout.err {
			assert.NotNil(t, err, "should error on '%s/%s' input", inout.typ, inout.val)
		} else {
			assert.Nil(t, err, "shouldn't error on '%s/%s' input", inout.typ, inout.val)
			assert.Equal(t, inout.out, out, "bad output for '%s/%s' input", inout.typ, inout.val)
		}
	}
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
