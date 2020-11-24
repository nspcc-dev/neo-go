package payload

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestGetBlockDataEncodeDecode(t *testing.T) {
	d := NewGetBlockByIndex(123, 100)
	testserdes.EncodeDecodeBinary(t, d, new(GetBlockByIndex))

	// invalid block count
	d = NewGetBlockByIndex(5, 0)
	data, err := testserdes.EncodeBinary(d)
	require.NoError(t, err)
	require.Error(t, testserdes.DecodeBinary(data, new(GetBlockByIndex)))

	// invalid block count
	d = NewGetBlockByIndex(5, MaxHeadersAllowed+1)
	data, err = testserdes.EncodeBinary(d)
	require.NoError(t, err)
	require.Error(t, testserdes.DecodeBinary(data, new(GetBlockByIndex)))
}
