package neofs

import (
	"errors"
	"net/url"
	"testing"

	cid "github.com/nspcc-dev/neofs-api-go/pkg/container/id"
	"github.com/nspcc-dev/neofs-api-go/pkg/object"
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
	containerID := cid.New()
	require.NoError(t, containerID.Parse(cStr))

	oStr := "3nQH1L8u3eM9jt2mZCs6MyjzdjerdSzBkXCYYj4M4Znk"
	oid := object.NewID()
	require.NoError(t, oid.Parse(oStr))

	validPrefix := "neofs:" + cStr + "/" + oStr
	objectAddr := object.NewAddress()
	objectAddr.SetContainerID(containerID)
	objectAddr.SetObjectID(oid)

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
				require.True(t, errors.Is(err, tc.err), "got: %#v", err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, objectAddr, oa)
			require.Equal(t, len(tc.params), len(ps))
			if len(ps) != 0 {
				require.Equal(t, tc.params, ps)
			}
		})
	}
}

func Test_checkUTF8(t *testing.T) {
	_, err := checkUTF8([]byte{0xFF})
	require.Error(t, err)

	a := []byte{1, 2, 3}
	b, err := checkUTF8(a)
	require.NoError(t, err)
	require.Equal(t, a, b)
}
