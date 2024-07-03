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

	t.Run("exceeds MaxArraySize", func(t *testing.T) {
		bytes := []byte{
			// The first byte represents the number 0x1.
			// It encodes the size of the outer array (the number or rows in the Nodes matrix).
			0x1,
			// This sequence of 9 bytes represents the number 0xffffffffffffffff.
			// It encodes the size of the first row in the Nodes matrix.
			// This size exceeds the maximum array size, thus the decoder should
			// return an error.
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		}
		require.Error(t, testserdes.DecodeBinary(bytes, new(MPTData)))
	})
}
