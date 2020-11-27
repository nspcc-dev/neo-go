package nef

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
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

	t.Run("zero-length script", func(t *testing.T) {
		expected.Script = make([]byte, 0)
		expected.Checksum = expected.CalculateChecksum()
		checkDecodeError(t, expected)
	})

	t.Run("invalid script length", func(t *testing.T) {
		newScript := make([]byte, MaxScriptLength+1)
		expected.Script = newScript
		expected.Checksum = expected.CalculateChecksum()
		checkDecodeError(t, expected)
	})

	t.Run("positive", func(t *testing.T) {
		expected.Script = script
		expected.Checksum = expected.CalculateChecksum()
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
		},
		Script: script,
	}
	expected.Checksum = expected.CalculateChecksum()

	bytes, err := expected.Bytes()
	require.NoError(t, err)
	actual, err := FileFromBytes(bytes)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestGetVersion(t *testing.T) {
	testCases := map[string]struct {
		input    string
		fails    bool
		expected Version
	}{
		"major only": {
			input: "1",
			fails: true,
		},
		"major and minor only": {
			input: "1.1",
			fails: true,
		},
		"major, minor and revision only": {
			input: "1.1.1",
			expected: Version{
				Major:    1,
				Minor:    1,
				Build:    1,
				Revision: 0,
			},
		},
		"full version": {
			input: "1.1.1.1",
			expected: Version{
				Major:    1,
				Minor:    1,
				Build:    1,
				Revision: 1,
			},
		},
		"dashed, without revision": {
			input: "1-pre.2-pre.3-pre",
			expected: Version{
				Major:    1,
				Minor:    2,
				Build:    3,
				Revision: 0,
			},
		},
		"dashed, full version": {
			input: "1-pre.2-pre.3-pre.4-pre",
			expected: Version{
				Major:    1,
				Minor:    2,
				Build:    3,
				Revision: 4,
			},
		},
		"dashed build": {
			input: "1.2.3-pre.4",
			expected: Version{
				Major:    1,
				Minor:    2,
				Build:    3,
				Revision: 4,
			},
		},
		"extra versions": {
			input: "1.2.3.4.5",
			fails: true,
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			actual, err := GetVersion(test.input)
			if test.fails {
				require.NotNil(t, err)
			} else {
				require.Equal(t, test.expected, actual)
			}
		})
	}
}
