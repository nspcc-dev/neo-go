package payload

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestMPTInventory_EncodeDecodeBinary(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		testserdes.EncodeDecodeBinary(t, NewMPTInventory([]util.Uint256{}), new(MPTInventory))
	})

	t.Run("good", func(t *testing.T) {
		inv := NewMPTInventory([]util.Uint256{{1, 2, 3}, {2, 3, 4}})
		testserdes.EncodeDecodeBinary(t, inv, new(MPTInventory))
	})

	t.Run("too large", func(t *testing.T) {
		check := func(t *testing.T, count int, fail bool) {
			h := make([]util.Uint256, count)
			for i := range h {
				h[i] = util.Uint256{1, 2, 3}
			}
			if fail {
				bytes, err := testserdes.EncodeBinary(NewMPTInventory(h))
				require.NoError(t, err)
				require.Error(t, testserdes.DecodeBinary(bytes, new(MPTInventory)))
			} else {
				testserdes.EncodeDecodeBinary(t, NewMPTInventory(h), new(MPTInventory))
			}
		}
		check(t, MaxMPTHashesCount, false)
		check(t, MaxMPTHashesCount+1, true)
	})
}
