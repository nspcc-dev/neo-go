package state

import (
	"math/rand"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestNEP5TransferLog_Append(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	expected := []*NEP5Transfer{
		randomTransfer(r),
		randomTransfer(r),
		randomTransfer(r),
		randomTransfer(r),
	}

	lg := new(NEP5TransferLog)
	for _, tr := range expected {
		require.NoError(t, lg.Append(tr))
	}

	require.Equal(t, len(expected), lg.Size())

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

	testserdes.EncodeDecodeBinary(t, expected, new(NEP5Tracker))
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

	testserdes.EncodeDecodeBinary(t, expected, new(NEP5Transfer))
}

func TestNEP5TransferSize(t *testing.T) {
	tr := randomTransfer(rand.New(rand.NewSource(0)))
	size := io.GetVarSize(tr)
	w := io.NewBufBinWriter()
	tr.EncodeBinary(w.BinWriter)
	require.NoError(t, w.Err)
	r := io.NewBinReaderFromBuf(w.Bytes())
	actualTr := &NEP5Transfer{}
	actual := actualTr.DecodeBinaryReturnCount(r)
	require.EqualValues(t, actual, size)
}

func randomTransfer(r *rand.Rand) *NEP5Transfer {
	return &NEP5Transfer{
		Amount: int64(r.Uint64()),
		Block:  r.Uint32(),
		Asset:  random.Uint160(),
		From:   random.Uint160(),
		To:     random.Uint160(),
		Tx:     random.Uint256(),
	}
}
