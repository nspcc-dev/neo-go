package neofs

import (
	"net/url"
	"testing"

	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/stretchr/testify/require"
)

func TestParseRange(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		r, err := parseRange("13|87")
		require.NoError(t, err)
		require.Equal(t, uint64(13), r.GetOffset())
		require.Equal(t, uint64(87), r.GetLength())
	})
	t.Run("missing offset", func(t *testing.T) {
		_, err := parseRange("|87")
		require.Error(t, err)
	})
	t.Run("missing length", func(t *testing.T) {
		_, err := parseRange("13|")
		require.Error(t, err)
	})
	t.Run("missing separator", func(t *testing.T) {
		_, err := parseRange("1387")
		require.Error(t, err)
	})
	t.Run("invalid number", func(t *testing.T) {
		_, err := parseRange("ab|87")
		require.Error(t, err)
	})
}

func TestParseNeoFSURL(t *testing.T) {
	cStr := "C3swfg8MiMJ9bXbeFG6dWJTCoHp9hAEZkHezvbSwK1Cc"
	oStr := "3nQH1L8u3eM9jt2mZCs6MyjzdjerdSzBkXCYYj4M4Znk"
	var objectAddr oid.Address
	require.NoError(t, objectAddr.DecodeString(cStr+"/"+oStr))

	validPrefix := "neofs:" + cStr + "/" + oStr

	testCases := []struct {
		url    string
		params []string
		err    error
	}{
		{validPrefix, nil, nil},
		{validPrefix + "/", []string{""}, nil},
		{validPrefix + "/range/1|2", []string{"range", "1|2"}, nil},
		{"neoffs:" + cStr + "/" + oStr, nil, ErrInvalidScheme},
		{"neofs:" + cStr, nil, ErrMissingObject},
		{"neofs:" + cStr + "ooo/" + oStr, nil, ErrInvalidContainer},
		{"neofs:" + cStr + "/ooo" + oStr, nil, ErrInvalidObject},
	}
	for _, tc := range testCases {
		t.Run(tc.url, func(t *testing.T) {
			u, err := url.Parse(tc.url)
			require.NoError(t, err)
			oa, ps, err := parseNeoFSURL(u)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, objectAddr, *oa)
			require.Equal(t, len(tc.params), len(ps))
			if len(ps) != 0 {
				require.Equal(t, tc.params, ps)
			}
		})
	}
}
