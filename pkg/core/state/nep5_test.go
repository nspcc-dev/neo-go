package state

import (
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestNEP5Tracker_EncodeBinary(t *testing.T) {
	expected := &NEP5Tracker{
		Balance:          int64(rand.Uint64()),
		LastUpdatedBlock: rand.Uint32(),
	}

	testEncodeDecode(t, expected, new(NEP5Tracker))
}

func TestNEP5Transfer_DecodeBinary(t *testing.T) {
	expected := &NEP5Transfer{
		Asset:     util.Uint160{1, 2, 3},
		From:      util.Uint160{5, 6, 7},
		To:        util.Uint160{8, 9, 10},
		Amount:    42,
		Block:     12345,
		Timestamp: 54321,
		Tx:        util.Uint256{8, 5, 3},
	}

	testEncodeDecode(t, expected, new(NEP5Transfer))
}

func testEncodeDecode(t *testing.T, expected, actual io.Serializable) {
	w := io.NewBufBinWriter()
	expected.EncodeBinary(w.BinWriter)
	require.NoError(t, w.Err)

	r := io.NewBinReaderFromBuf(w.Bytes())
	actual.DecodeBinary(r)
	require.NoError(t, r.Err)
	require.Equal(t, expected, actual)
}
