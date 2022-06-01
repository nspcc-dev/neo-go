package transaction

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
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
		for _, code := range []OracleResponseCode{ProtocolNotSupported, ConsensusUnreachable,
			NotFound, Timeout, Forbidden, ResponseTooLarge, InsufficientFunds, Error} {
			attr = &Attribute{
				Type: OracleResponseT,
				Value: &OracleResponse{
					ID:     42,
					Code:   code,
					Result: []byte{},
				},
			}
			testserdes.EncodeDecodeBinary(t, attr, new(Attribute))
		}
	})
	t.Run("NotValidBefore", func(t *testing.T) {
		t.Run("positive", func(t *testing.T) {
			attr := &Attribute{
				Type: NotValidBeforeT,
				Value: &NotValidBefore{
					Height: 123,
				},
			}
			testserdes.EncodeDecodeBinary(t, attr, new(Attribute))
		})
		t.Run("bad format: too short", func(t *testing.T) {
			bw := io.NewBufBinWriter()
			bw.WriteBytes([]byte{1, 2, 3})
			require.Error(t, testserdes.DecodeBinary(bw.Bytes(), new(NotValidBefore)))
		})
	})
	t.Run("Reserved", func(t *testing.T) {
		getReservedAttribute := func(t AttrType) *Attribute {
			return &Attribute{
				Type: t,
				Value: &Reserved{
					Value: []byte{1, 2, 3, 4, 5},
				},
			}
		}
		t.Run("lower bound", func(t *testing.T) {
			testserdes.EncodeDecodeBinary(t, getReservedAttribute(ReservedLowerBound+3), new(Attribute))
		})
		t.Run("upper bound", func(t *testing.T) {
			testserdes.EncodeDecodeBinary(t, getReservedAttribute(ReservedUpperBound), new(Attribute))
		})
		t.Run("inside bounds", func(t *testing.T) {
			testserdes.EncodeDecodeBinary(t, getReservedAttribute(ReservedLowerBound+5), new(Attribute))
		})
		t.Run("not reserved", func(t *testing.T) {
			_, err := testserdes.EncodeBinary(getReservedAttribute(ReservedLowerBound - 1))
			require.Error(t, err)
		})
	})
	t.Run("Conflicts", func(t *testing.T) {
		t.Run("positive", func(t *testing.T) {
			attr := &Attribute{
				Type: ConflictsT,
				Value: &Conflicts{
					Hash: random.Uint256(),
				},
			}
			testserdes.EncodeDecodeBinary(t, attr, new(Attribute))
		})
		t.Run("negative: bad uint256", func(t *testing.T) {
			bw := io.NewBufBinWriter()
			bw.WriteBytes(make([]byte, util.Uint256Size-1))
			require.Error(t, testserdes.DecodeBinary(bw.Bytes(), new(Conflicts)))
		})
	})
	t.Run("NotaryAssisted", func(t *testing.T) {
		t.Run("positive", func(t *testing.T) {
			attr := &Attribute{
				Type: NotaryAssistedT,
				Value: &NotaryAssisted{
					NKeys: 3,
				},
			}
			testserdes.EncodeDecodeBinary(t, attr, new(Attribute))
		})
		t.Run("bad format: too short", func(t *testing.T) {
			bw := io.NewBufBinWriter()
			bw.WriteBytes([]byte{})
			require.Error(t, testserdes.DecodeBinary(bw.Bytes(), new(NotaryAssisted)))
		})
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
			"code": "Success",
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
	t.Run("Conflicts", func(t *testing.T) {
		attr := &Attribute{
			Type: ConflictsT,
			Value: &Conflicts{
				Hash: random.Uint256(),
			},
		}
		testserdes.MarshalUnmarshalJSON(t, attr, new(Attribute))
	})
	t.Run("NotaryAssisted", func(t *testing.T) {
		attr := &Attribute{
			Type: NotaryAssistedT,
			Value: &NotaryAssisted{
				NKeys: 3,
			},
		}
		testserdes.MarshalUnmarshalJSON(t, attr, new(Attribute))
	})
}
