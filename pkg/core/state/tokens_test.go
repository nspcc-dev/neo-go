package state

import (
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestTokenTransferLog_Append17(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 100500))
	expected := []*NEP17Transfer{
		random17Transfer(r),
		random17Transfer(r),
		random17Transfer(r),
		random17Transfer(r),
	}

	lg := new(TokenTransferLog)
	for _, tr := range expected {
		require.NoError(t, lg.Append(tr))
	}

	require.Equal(t, len(expected), lg.Size())

	i := len(expected) - 1
	cont, err := lg.ForEachNEP17(func(tr *NEP17Transfer) (bool, error) {
		require.Equal(t, expected[i], tr)
		i--
		return true, nil
	})
	require.NoError(t, err)
	require.True(t, cont)
}

func TestTokenTransferLog_Append(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 100500))
	expected := []*NEP11Transfer{
		random11Transfer(r),
		random11Transfer(r),
		random11Transfer(r),
		random11Transfer(r),
	}

	lg := new(TokenTransferLog)
	for _, tr := range expected {
		require.NoError(t, lg.Append(tr))
	}

	require.Equal(t, len(expected), lg.Size())

	i := len(expected) - 1
	cont, err := lg.ForEachNEP11(func(tr *NEP11Transfer) (bool, error) {
		require.Equal(t, expected[i], tr)
		i--
		return true, nil
	})
	require.NoError(t, err)
	require.True(t, cont)
}

func BenchmarkTokenTransferLog_Append(b *testing.B) {
	r := rand.New(rand.NewPCG(42, 100500))
	ts := make([]*NEP17Transfer, TokenTransferBatchSize)
	for i := range ts {
		ts[i] = random17Transfer(r)
	}

	lg := new(TokenTransferLog)
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		for _, tr := range ts {
			err := lg.Append(tr)
			if err != nil {
				b.FailNow()
			}
		}
	}
}

func TestNEP17Transfer_DecodeBinary(t *testing.T) {
	expected := &NEP17Transfer{
		Asset:        123,
		Counterparty: util.Uint160{5, 6, 7},
		Amount:       big.NewInt(42),
		Block:        12345,
		Timestamp:    54321,
		Tx:           util.Uint256{8, 5, 3},
	}

	testserdes.EncodeDecodeBinary(t, expected, new(NEP17Transfer))
}

func TestNEP11Transfer_DecodeBinary(t *testing.T) {
	expected := &NEP11Transfer{
		NEP17Transfer: NEP17Transfer{
			Asset:        123,
			Counterparty: util.Uint160{5, 6, 7},
			Amount:       big.NewInt(42),
			Block:        12345,
			Timestamp:    54321,
			Tx:           util.Uint256{8, 5, 3},
		},
		ID: []byte{42, 42, 42},
	}

	testserdes.EncodeDecodeBinary(t, expected, new(NEP11Transfer))
}

func random17Transfer(r *rand.Rand) *NEP17Transfer {
	return &NEP17Transfer{
		Amount:       big.NewInt(int64(r.Uint64())),
		Block:        r.Uint32(),
		Asset:        int32(random.Int(10, 10000000)),
		Counterparty: random.Uint160(),
		Tx:           random.Uint256(),
	}
}

func random11Transfer(r *rand.Rand) *NEP11Transfer {
	return &NEP11Transfer{
		NEP17Transfer: NEP17Transfer{
			Amount:       big.NewInt(int64(r.Uint64())),
			Block:        r.Uint32(),
			Asset:        int32(random.Int(10, 10000000)),
			Counterparty: random.Uint160(),
			Tx:           random.Uint256(),
		},
		ID: random.Uint256().BytesBE(),
	}
}
