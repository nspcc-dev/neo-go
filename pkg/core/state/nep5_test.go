package state

import (
	gio "io"
	"math/rand"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestNEP5TransferLog_Append(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	expected := []*NEP5Transfer{
		randomTransfer(t, r),
		randomTransfer(t, r),
		randomTransfer(t, r),
		randomTransfer(t, r),
	}

	lg := new(NEP5TransferLog)
	for _, tr := range expected {
		require.NoError(t, lg.Append(tr))
	}

	i := 0
	err := lg.ForEach(func(tr *NEP5Transfer) error {
		require.Equal(t, expected[i], tr)
		i++
		return nil
	})
	require.NoError(t, err)

}

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

func TestNEP5TransferSize(t *testing.T) {
	tr := randomTransfer(t, rand.New(rand.NewSource(0)))
	size := io.GetVarSize(tr)
	require.EqualValues(t, NEP5TransferSize, size)
}

func randomTransfer(t *testing.T, r *rand.Rand) *NEP5Transfer {
	tr := &NEP5Transfer{
		Amount: int64(r.Uint64()),
		Block:  r.Uint32(),
	}

	var err error
	_, err = gio.ReadFull(r, tr.Asset[:])
	require.NoError(t, err)
	_, err = gio.ReadFull(r, tr.From[:])
	require.NoError(t, err)
	_, err = gio.ReadFull(r, tr.To[:])
	require.NoError(t, err)
	_, err = gio.ReadFull(r, tr.Tx[:])
	require.NoError(t, err)

	return tr
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
