package nef

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeBinary(t *testing.T) {
	script := []byte{12, 32, 84, 35, 14}
	expected := &File{
		Header: Header{
			Magic:    Magic,
			Compiler: "best compiler version 1",
		},
		Tokens: []MethodToken{{
			Hash:       random.Uint160(),
			Method:     "method",
			ParamCount: 3,
			HasReturn:  true,
			CallFlag:   callflag.WriteStates,
		}},
		Script: script,
	}

	t.Run("invalid Magic", func(t *testing.T) {
		expected.Magic = 123
		checkDecodeError(t, expected)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		expected.Magic = Magic
		expected.Checksum = 123
		checkDecodeError(t, expected)
	})

	t.Run("zero-length script", func(t *testing.T) {
		expected.Script = make([]byte, 0)
		expected.Checksum = expected.CalculateChecksum()
		checkDecodeError(t, expected)
	})

	t.Run("invalid script length", func(t *testing.T) {
		newScript := make([]byte, stackitem.MaxSize+1)
		expected.Script = newScript
		expected.Checksum = expected.CalculateChecksum()
		checkDecodeError(t, expected)
	})
	t.Run("invalid tokens list", func(t *testing.T) {
		expected.Script = script
		expected.Tokens[0].Method = "_reserved"
		expected.Checksum = expected.CalculateChecksum()
		checkDecodeError(t, expected)
	})

	t.Run("positive", func(t *testing.T) {
		expected.Script = script
		expected.Tokens[0].Method = "method"
		expected.Checksum = expected.CalculateChecksum()
		expected.Magic = Magic
		testserdes.EncodeDecodeBinary(t, expected, &File{})
	})
	t.Run("positive with empty tokens", func(t *testing.T) {
		expected.Script = script
		// Check `Tokens` won't be deserialized to `nil`. We expect them to be non-nil slice in the related code.
		expected.Tokens = []MethodToken{}
		expected.Checksum = expected.CalculateChecksum()
		expected.Magic = Magic
		testserdes.EncodeDecodeBinary(t, expected, &File{})
	})
	t.Run("invalid reserved bytes", func(t *testing.T) {
		expected.Script = script
		expected.Tokens = expected.Tokens[:0]
		expected.Checksum = expected.CalculateChecksum()
		bytes, err := testserdes.EncodeBinary(expected)
		require.NoError(t, err)

		sz := io.GetVarSize(&expected.Header)
		bytes[sz] = 1
		err = testserdes.DecodeBinary(bytes, new(File))
		require.ErrorIs(t, err, errInvalidReserved)

		bytes[sz] = 0
		bytes[sz+3] = 1
		err = testserdes.DecodeBinary(bytes, new(File))
		require.ErrorIs(t, err, errInvalidReserved)
	})
}

func checkDecodeError(t *testing.T, expected *File) {
	bytes, err := testserdes.EncodeBinary(expected)
	require.NoError(t, err)
	require.Error(t, testserdes.DecodeBinary(bytes, &File{}))
}

func TestBytesFromBytes(t *testing.T) {
	script := []byte{12, 32, 84, 35, 14}
	expected := File{
		Header: Header{
			Magic:    Magic,
			Compiler: "best compiler version 1",
		},
		Tokens: []MethodToken{{
			Hash:       random.Uint160(),
			Method:     "someMethod",
			ParamCount: 3,
			HasReturn:  true,
			CallFlag:   callflag.WriteStates,
		}},
		Script: script,
	}
	expected.Checksum = expected.CalculateChecksum()

	bytes, err := expected.Bytes()
	require.NoError(t, err)
	actual, err := FileFromBytes(bytes)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestNewFileFromBytesLimits(t *testing.T) {
	expected := File{
		Header: Header{
			Magic:    Magic,
			Compiler: "best compiler version 1",
		},
		Tokens: []MethodToken{{
			Hash:       random.Uint160(),
			Method:     "someMethod",
			ParamCount: 3,
			HasReturn:  true,
			CallFlag:   callflag.WriteStates,
		}},
		Script: make([]byte, stackitem.MaxSize-100),
	}
	expected.Checksum = expected.CalculateChecksum()

	bytes, err := expected.BytesLong()
	require.NoError(t, err)
	_, err = FileFromBytes(bytes)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid NEF file size")
}

func TestMarshalUnmarshalJSON(t *testing.T) {
	expected := &File{
		Header: Header{
			Magic:    Magic,
			Compiler: "test.compiler-test.ver",
		},
		Tokens: []MethodToken{{
			Hash:       util.Uint160{0x12, 0x34, 0x56, 0x78, 0x91, 0x00},
			Method:     "someMethod",
			ParamCount: 3,
			HasReturn:  true,
			CallFlag:   callflag.WriteStates,
		}},
		Script: []byte{1, 2, 3, 4},
	}
	expected.Checksum = expected.CalculateChecksum()

	data, err := json.Marshal(expected)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"magic":`+strconv.FormatUint(uint64(Magic), 10)+`,
		"compiler": "test.compiler-test.ver",
		"source": "",
		"tokens": [
			{
	"hash": "0x`+expected.Tokens[0].Hash.StringLE()+`",
	"method": "someMethod",
	"paramcount": 3,
	"hasreturnvalue": true,
	"callflags": "`+expected.Tokens[0].CallFlag.String()+`"
			}
		],
		"script": "`+base64.StdEncoding.EncodeToString(expected.Script)+`",
		"checksum":`+strconv.FormatUint(uint64(expected.Checksum), 10)+`}`, string(data))

	actual := new(File)
	require.NoError(t, json.Unmarshal(data, actual))
	require.Equal(t, expected, actual)
}
