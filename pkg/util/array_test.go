package util

import (
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

func TestArrayReverse(t *testing.T) {
	for _, tc := range testCases {
		have := ArrayReverse(tc.arr)
		require.Equal(t, tc.rev, have)
	}
}
