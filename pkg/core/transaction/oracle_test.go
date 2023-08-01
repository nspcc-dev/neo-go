package transaction

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestOracleResponse_EncodeBinary(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		r := &OracleResponse{
			ID:     rand.Uint64(),
			Code:   Success,
			Result: []byte{1, 2, 3, 4, 5},
		}
		testserdes.EncodeDecodeBinary(t, r, new(OracleResponse))
	})
	t.Run("ErrorCodes", func(t *testing.T) {
		codes := []OracleResponseCode{NotFound, Timeout, Forbidden, Error}
		for _, c := range codes {
			r := &OracleResponse{
				ID:     rand.Uint64(),
				Code:   c,
				Result: []byte{},
			}
			testserdes.EncodeDecodeBinary(t, r, new(OracleResponse))
		}
	})
	t.Run("Error", func(t *testing.T) {
		t.Run("InvalidCode", func(t *testing.T) {
			r := &OracleResponse{
				ID:     rand.Uint64(),
				Code:   0x42,
				Result: []byte{},
			}
			bs, err := testserdes.EncodeBinary(r)
			require.NoError(t, err)

			err = testserdes.DecodeBinary(bs, new(OracleResponse))
			require.ErrorIs(t, err, ErrInvalidResponseCode)
		})
		t.Run("InvalidResult", func(t *testing.T) {
			r := &OracleResponse{
				ID:     rand.Uint64(),
				Code:   Error,
				Result: []byte{1},
			}
			bs, err := testserdes.EncodeBinary(r)
			require.NoError(t, err)

			err = testserdes.DecodeBinary(bs, new(OracleResponse))
			require.ErrorIs(t, err, ErrInvalidResult)
		})
	})
}

func TestOracleResponse_toJSONMap(t *testing.T) {
	r := &OracleResponse{
		ID:     rand.Uint64(),
		Code:   Success,
		Result: []byte{1},
	}

	b1, err := json.Marshal(r)
	require.NoError(t, err)

	m := map[string]any{}
	r.toJSONMap(m)
	b2, err := json.Marshal(m)
	require.NoError(t, err)
	require.JSONEq(t, string(b1), string(b2))
}
