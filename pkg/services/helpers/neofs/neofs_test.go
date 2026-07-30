package neofs

import (
	"net/url"
	"testing"

	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/stretchr/testify/require"
)

func TestParseNeoFSURL(t *testing.T) {
	cStr := "C3swfg8MiMJ9bXbeFG6dWJTCoHp9hAEZkHezvbSwK1Cc"
	oStr := "3nQH1L8u3eM9jt2mZCs6MyjzdjerdSzBkXCYYj4M4Znk"
	var objectAddr oid.Address
	require.NoError(t, objectAddr.DecodeString(cStr+"/"+oStr))

	validPrefix := "neofs:" + cStr + "/" + oStr

	testCases := []struct {
		url  string
		rest string
		err  error
	}{
		{validPrefix, "", nil},
		{validPrefix + "/", "", nil},
		{validPrefix + "/range/1|2", "range/1|2", nil},
		{"neoffs:" + cStr + "/" + oStr, "", ErrInvalidScheme},
		{"neofs:" + cStr, "", ErrMissingObject},
		{"neofs:" + cStr + "ooo/" + oStr, "", ErrInvalidContainer},
		{"neofs:" + cStr + "/ooo" + oStr, "", ErrInvalidObject},
	}
	for _, tc := range testCases {
		t.Run(tc.url, func(t *testing.T) {
			u, err := url.Parse(tc.url)
			require.NoError(t, err)
			oa, rest, err := parseNeoFSURL(u)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, objectAddr, *oa)
			require.Equal(t, tc.rest, rest)
		})
	}
}
