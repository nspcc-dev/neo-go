package nef

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeBinary(t *testing.T) {
	script := []byte{12, 32, 84, 35, 14}
	expected := &File{
		Header: Header{
			Magic:    Magic,
			Compiler: "the best compiler ever",
			Version: Version{
				Major:    1,
				Minor:    2,
				Build:    3,
				Revision: 4,
			},
			ScriptHash: hash.Hash160(script),
		},
		Script: script,
	}

	t.Run("invalid Magic", func(t *testing.T) {
		expected.Header.Magic = 123
		checkDecodeError(t, expected)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		expected.Header.Magic = Magic
		expected.Checksum = 123
		checkDecodeError(t, expected)
	})

	t.Run("invalid script length", func(t *testing.T) {
		newScript := make([]byte, MaxScriptLength+1)
		expected.Script = newScript
		expected.Header.ScriptHash = hash.Hash160(newScript)
		expected.Checksum = expected.Header.CalculateChecksum()
		checkDecodeError(t, expected)
	})

	t.Run("invalid scripthash", func(t *testing.T) {
		expected.Script = script
		expected.Header.ScriptHash = util.Uint160{1, 2, 3}
		expected.Checksum = expected.Header.CalculateChecksum()
		checkDecodeError(t, expected)
	})

	t.Run("positive", func(t *testing.T) {
		expected.Script = script
		expected.Header.ScriptHash = hash.Hash160(script)
		expected.Checksum = expected.Header.CalculateChecksum()
		expected.Header.Magic = Magic
		testserdes.EncodeDecodeBinary(t, expected, &File{})
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
			Compiler: "the best compiler ever",
			Version: Version{
				Major:    1,
				Minor:    2,
				Build:    3,
				Revision: 4,
			},
			ScriptHash: hash.Hash160(script),
		},
		Script: script,
	}
	expected.Checksum = expected.Header.CalculateChecksum()

	bytes, err := expected.Bytes()
	require.NoError(t, err)
	actual, err := FileFromBytes(bytes)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestGetVersion(t *testing.T) {
	_, err := GetVersion("qwerty")
	require.Error(t, err)

	_, err = GetVersion("1.pre")
	require.Error(t, err)

	_, err = GetVersion("1.1.pre")
	require.Error(t, err)

	_, err = GetVersion("1.1.1.pre")
	require.Error(t, err)

	actual, err := GetVersion("1.1.1-pre")
	require.NoError(t, err)
	expected := Version{
		Major:    1,
		Minor:    1,
		Build:    1,
		Revision: 0,
	}
	require.Equal(t, expected, actual)

	actual, err = GetVersion("0.90.0-pre")
	require.NoError(t, err)
	expected = Version{
		Major:    0,
		Minor:    90,
		Build:    0,
		Revision: 0,
	}
	require.Equal(t, expected, actual)

	actual, err = GetVersion("1.1.1.1-pre")
	require.NoError(t, err)
	expected = Version{
		Major:    1,
		Minor:    1,
		Build:    1,
		Revision: 1,
	}
	require.Equal(t, expected, actual)

	_, err = GetVersion("1.1.1.1.1")
	require.Error(t, err)
}
