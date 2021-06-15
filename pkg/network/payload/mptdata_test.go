package payload

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestMPTData_EncodeDecodeBinary(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		d := new(MPTData)
		bytes, err := testserdes.EncodeBinary(d)
		require.NoError(t, err)
		require.Error(t, testserdes.DecodeBinary(bytes, new(MPTData)))
	})

	t.Run("good", func(t *testing.T) {
		d := &MPTData{
			Nodes: [][]byte{{}, {1}, {1, 2, 3}},
		}
		testserdes.EncodeDecodeBinary(t, d, new(MPTData))
	})
}
