package transaction

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestAttribute_EncodeBinary(t *testing.T) {
	t.Run("HighPriority", func(t *testing.T) {
		attr := &Attribute{
			Type: HighPriority,
		}
		testserdes.EncodeDecodeBinary(t, attr, new(Attribute))
	})
	t.Run("OracleResponse", func(t *testing.T) {
		attr := &Attribute{
			Type: OracleResponseT,
			Value: &OracleResponse{
				ID:     0x1122334455,
				Code:   Success,
				Result: []byte{1, 2, 3},
			},
		}
		testserdes.EncodeDecodeBinary(t, attr, new(Attribute))
	})
	t.Run("NotValidBefore", func(t *testing.T) {
		attr := &Attribute{
			Type: NotValidBeforeT,
			Value: &NotValidBefore{
				Height: 123,
			},
		}
		testserdes.EncodeDecodeBinary(t, attr, new(Attribute))
	})
}

func TestAttribute_MarshalJSON(t *testing.T) {
	t.Run("HighPriority", func(t *testing.T) {
		attr := &Attribute{Type: HighPriority}
		data, err := json.Marshal(attr)
		require.NoError(t, err)
		require.JSONEq(t, `{"type":"HighPriority"}`, string(data))

		actual := new(Attribute)
		require.NoError(t, json.Unmarshal(data, actual))
		require.Equal(t, attr, actual)
	})
	t.Run("OracleResponse", func(t *testing.T) {
		res := []byte{1, 2, 3}
		attr := &Attribute{
			Type: OracleResponseT,
			Value: &OracleResponse{
				ID:     123,
				Code:   Success,
				Result: res,
			},
		}
		data, err := json.Marshal(attr)
		require.NoError(t, err)
		require.JSONEq(t, `{
			"type":"OracleResponse",
			"id": 123,
			"code": 0,
			"result": "`+base64.StdEncoding.EncodeToString(res)+`"}`, string(data))

		actual := new(Attribute)
		require.NoError(t, json.Unmarshal(data, actual))
		require.Equal(t, attr, actual)
		testserdes.EncodeDecodeBinary(t, attr, new(Attribute))
	})
	t.Run("NotValidBefore", func(t *testing.T) {
		attr := &Attribute{
			Type: NotValidBeforeT,
			Value: &NotValidBefore{
				Height: 123,
			},
		}
		testserdes.MarshalUnmarshalJSON(t, attr, new(Attribute))
	})
}
