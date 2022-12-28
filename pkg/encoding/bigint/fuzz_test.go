//go:build go1.18
// +build go1.18

package bigint

import (
	"bytes"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzFromBytes(f *testing.F) {
	for _, tc := range testCases {
		f.Add(tc.buf)
	}
	for i := 0; i < 50; i++ {
		for j := 1; j < MaxBytesLen; j++ {
			b := make([]byte, j)
			_, err := rand.Read(b)
			require.NoError(f, err)
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		var bi *big.Int
		require.NotPanics(t, func() { bi = FromBytes(raw) })

		var actual []byte
		require.NotPanics(t, func() { actual = ToBytes(bi) })
		require.True(t, len(actual) <= len(raw), "actual: %x, raw: %x", actual, raw)

		require.True(t, bytes.Equal(actual, raw[:len(actual)]), "actual: %x, raw: %x", actual, raw)
		if len(actual) == len(raw) {
			return
		}

		var b byte
		if bi.Sign() == -1 {
			b = 0xFF
		}
		for i := len(actual); i < len(raw); i++ {
			require.Equal(t, b, raw[i], "invalid prefix")
		}

		newRaw := ToBytes(bi)
		newBi := FromBytes(newRaw)
		require.Equal(t, bi, newBi)
	})
}
