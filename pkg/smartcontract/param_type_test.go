package smartcontract

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		out: ArrayType,
	}, {
		in:  "map",
		out: MapType,
	}, {
		in:  "interopinterface",
		out: InteropInterfaceType,
	}, {
		in:  "void",
		out: VoidType,
	}, {
		in:  "qwerty",
		err: true,
	}}
	for _, inout := range inouts {
		out, err := ParseParamType(inout.in)
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
		in:  "NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8",
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
		out: mustHex("602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b"),
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
		out: int64(0),
	}, {
		typ: IntegerType,
		val: "42",
		out: int64(42),
	}, {
		typ: IntegerType,
		val: "-42",
		out: int64(-42),
	}, {
		typ: IntegerType,
		val: "q",
		err: true,
	}, {
		typ: Hash160Type,
		val: "NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8",
		out: util.Uint160{
			0x23, 0xba, 0x27, 0x3, 0xc5, 0x32, 0x63, 0xe8, 0xd6, 0xe5,
			0x22, 0xdc, 0x32, 0x20, 0x33, 0x39, 0xdc, 0xd8, 0xee, 0xe9,
		},
	}, {
		typ: Hash160Type,
		val: "50befd26fdf6e4d957c11e078b24ebce6291456f",
		out: util.Uint160{
			0x6f, 0x45, 0x91, 0x62, 0xce, 0xeb, 0x24, 0x8b, 0x7, 0x1e,
			0xc1, 0x57, 0xd9, 0xe4, 0xf6, 0xfd, 0x26, 0xfd, 0xbe, 0x50,
		},
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
		out: util.Uint256{
			0xe7, 0x2d, 0x28, 0x69, 0x79, 0xee, 0x6c, 0xb1, 0xb7, 0xe6, 0x5d, 0xfd, 0xdf, 0xb2, 0xe3, 0x84,
			0x10, 0xb, 0x8d, 0x14, 0x8e, 0x77, 0x58, 0xde, 0x42, 0xe4, 0x16, 0x8b, 0x71, 0x79, 0x2c, 0x60,
		},
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
		out: mustHex("602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282d"),
	}, {
		typ: ByteArrayType,
		val: "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7",
		out: mustHex("602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"),
	}, {
		typ: ByteArrayType,
		val: "50befd26fdf6e4d957c11e078b24ebce6291456f",
		out: mustHex("50befd26fdf6e4d957c11e078b24ebce6291456f"),
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
		out: mustHex("ab"),
	}, {
		typ: PublicKeyType,
		val: "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c",
		out: mustHex("03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"),
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
	}, {
		typ: MapType,
		val: "[]",
		err: true,
	}, {
		typ: InteropInterfaceType,
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

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func TestConvertToParamType(t *testing.T) {
	for _, expected := range []ParamType{
		UnknownType,
		AnyType,
		BoolType,
		IntegerType,
		ByteArrayType,
		StringType,
		Hash160Type,
		Hash256Type,
		PublicKeyType,
		SignatureType,
		ArrayType,
		MapType,
		InteropInterfaceType,
		VoidType,
	} {
		actual, err := ConvertToParamType(int(expected))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}

	_, err := ConvertToParamType(0x01)
	require.NotNil(t, err)
}
