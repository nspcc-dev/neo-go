package state

import (
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestNEP17TransferLog_Append(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	expected := []*NEP17Transfer{
		randomTransfer(r),
		randomTransfer(r),
		randomTransfer(r),
		randomTransfer(r),
	}

	lg := new(NEP17TransferLog)
	for _, tr := range expected {
		require.NoError(t, lg.Append(tr))
	}

	require.Equal(t, len(expected), lg.Size())

	i := len(expected) - 1
	cont, err := lg.ForEach(func(tr *NEP17Transfer) (bool, error) {
		require.Equal(t, expected[i], tr)
		i--
		return true, nil
	})
	require.NoError(t, err)
	require.True(t, cont)
}

func TestNEP17Transfer_DecodeBinary(t *testing.T) {
	expected := &NEP17Transfer{
		Asset:     123,
		From:      util.Uint160{5, 6, 7},
		To:        util.Uint160{8, 9, 10},
		Amount:    *big.NewInt(42),
		Block:     12345,
		Timestamp: 54321,
		Tx:        util.Uint256{8, 5, 3},
	}

	testserdes.EncodeDecodeBinary(t, expected, new(NEP17Transfer))
}

func randomTransfer(r *rand.Rand) *NEP17Transfer {
	return &NEP17Transfer{
		Amount: *big.NewInt(int64(r.Uint64())),
		Block:  r.Uint32(),
		Asset:  int32(random.Int(10, 10000000)),
		From:   random.Uint160(),
		To:     random.Uint160(),
		Tx:     random.Uint256(),
	}
}
