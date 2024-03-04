package slice

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var testCases = []struct {
	arr []byte
	rev []byte
}{
	{
		arr: []byte{},
		rev: []byte{},
	},
	{
		arr: []byte{0x01},
		rev: []byte{0x01},
	},
	{
		arr: []byte{0x01, 0x02, 0x03, 0x04},
		rev: []byte{0x04, 0x03, 0x02, 0x01},
	},
	{
		arr: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
		rev: []byte{0x05, 0x04, 0x03, 0x02, 0x01},
	},
}

func TestCopyReverse(t *testing.T) {
	for _, tc := range testCases {
		arg := bytes.Clone(tc.arr)
		require.Equal(t, tc.arr, arg)

		have := CopyReverse(arg)
		require.Equal(t, tc.rev, have)

		// test that argument was copied
		for i := range have {
			have[i] = ^have[i]
		}
		require.Equal(t, tc.arr, arg)

		Reverse(arg)
		require.Equal(t, tc.rev, arg)
		if len(tc.arr) > 1 {
			require.NotEqual(t, tc.arr, arg)
		}
	}
}

func TestClean(t *testing.T) {
	for _, tc := range testCases[1:] { // Empty one will be equal.
		cp := bytes.Clone(tc.arr)
		Clean(cp)
		require.NotEqual(t, tc.arr, cp)
	}
}
