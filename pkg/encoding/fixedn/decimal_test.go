package fixedn

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecimalFromStringGood(t *testing.T) {
	var testCases = []struct {
		bi   *big.Int
		prec int
		s    string
	}{
		{big.NewInt(123), 2, "1.23"},
		{big.NewInt(12300), 2, "123"},
		{big.NewInt(1234500000), 8, "12.345"},
		{big.NewInt(-12345), 3, "-12.345"},
		{big.NewInt(35), 8, "0.00000035"},
		{big.NewInt(1230), 5, "0.0123"},
		{big.NewInt(123456789), 20, "0.00000000000123456789"},
	}
	for _, tc := range testCases {
		t.Run(tc.s, func(t *testing.T) {
			s := ToString(tc.bi, tc.prec)
			require.Equal(t, tc.s, s)

			bi, err := FromString(s, tc.prec)
			require.NoError(t, err)
			require.Equal(t, tc.bi, bi)
		})
	}
}

func TestDecimalFromStringBad(t *testing.T) {
	var errCases = []struct {
		s    string
		prec int
	}{
		{"", 0},
		{"", 1},
		{"12A", 1},
		{"12.345", 2},
		{"12.3A", 2},
	}
	for _, tc := range errCases {
		t.Run(tc.s, func(t *testing.T) {
			_, err := FromString(tc.s, tc.prec)
			require.Error(t, err)
		})
	}
}
