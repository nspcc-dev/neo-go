package payload

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestGetBlockDataEncodeDecode(t *testing.T) {
	d := NewGetBlockData(123, 100)
	testserdes.EncodeDecodeBinary(t, d, new(GetBlockData))

	// invalid block count
	d = NewGetBlockData(5, 0)
	data, err := testserdes.EncodeBinary(d)
	require.NoError(t, err)
	require.Error(t, testserdes.DecodeBinary(data, new(GetBlockData)))

	// invalid block count
	d = NewGetBlockData(5, maxBlockCount+1)
	data, err = testserdes.EncodeBinary(d)
	require.NoError(t, err)
	require.Error(t, testserdes.DecodeBinary(data, new(GetBlockData)))
}
