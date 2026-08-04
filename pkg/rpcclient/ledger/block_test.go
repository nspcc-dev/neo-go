package ledger

import (
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newTestBlock(stateRootEnabled bool) *Block {
	b := &Block{
		Hash:               random.Uint256(),
		Version:            0,
		PrevHash:           random.Uint256(),
		MerkleRoot:         random.Uint256(),
		Timestamp:          100500,
		Nonce:              123,
		Index:              1,
		PrimaryIndex:       3,
		NextConsensus:      random.Uint160(),
		TransactionsLength: 2,
	}
	if stateRootEnabled {
		b.StateRootEnabled = true
		b.PrevStateRoot = random.Uint256()
	}
	return b
}

func TestBlock_ToFromStackItem(t *testing.T) {
	check := func(t *testing.T, stateRootEnabled bool) {
		b := newTestBlock(stateRootEnabled)
		actual := &Block{StateRootEnabled: stateRootEnabled}
		testserdes.ToFromStackItem(t, b, actual)
	}
	t.Run("StateRoot enabled", func(t *testing.T) {
		check(t, true)
	})
	t.Run("StateRoot disabled", func(t *testing.T) {
		check(t, false)
	})

	t.Run("StateRootEnabled mismatch", func(t *testing.T) {
		checkMismatch := func(t *testing.T, sourceStateRootEnabled bool) {
			b := newTestBlock(sourceStateRootEnabled)
			item, err := b.ToStackItem()
			require.NoError(t, err)

			actual := &Block{StateRootEnabled: !sourceStateRootEnabled}
			require.ErrorContains(t, actual.FromStackItem(item), "wrong number of structure elements")
		}
		t.Run("source has state root, receiver doesn't", func(t *testing.T) { checkMismatch(t, true) })
		t.Run("source doesn't have state root, receiver does", func(t *testing.T) { checkMismatch(t, false) })
	})
}

func TestBlockFromStackItem(t *testing.T) {
	var (
		bHash         = stackitem.NewByteArray(random.Uint256().BytesBE())
		version       = stackitem.Make(0)
		prevHash      = stackitem.NewByteArray(random.Uint256().BytesBE())
		merkleRoot    = stackitem.NewByteArray(random.Uint256().BytesBE())
		timestamp     = stackitem.Make(100500)
		nonce         = stackitem.Make(123)
		index         = stackitem.Make(1)
		primaryIndex  = stackitem.Make(0)
		nextConsensus = stackitem.NewByteArray(random.Uint160().BytesBE())
		txLen         = stackitem.Make(2)
		prevStateRoot = stackitem.NewByteArray(random.Uint256().BytesBE())

		good = []stackitem.Item{bHash, version, prevHash, merkleRoot, timestamp, nonce, index, primaryIndex, nextConsensus, txLen}

		badCases = []struct {
			name             string
			item             stackitem.Item
			stateRootEnabled bool
		}{
			{"not an array", stackitem.Make(1), false},
			{"wrong number of elements", stackitem.NewArray(good[:9]), false},
			{"hash is not a byte string", stackitem.NewArray(replaceAt(good, 0, stackitem.NewArray(nil))), false},
			{"hash is not a hash", stackitem.NewArray(replaceAt(good, 0, stackitem.NewByteArray([]byte{1, 2, 3}))), false},
			{"version is not a number", stackitem.NewArray(replaceAt(good, 1, stackitem.NewArray(nil))), false},
			{"version is out of range", stackitem.NewArray(replaceAt(good, 1, stackitem.Make(-1))), false},
			{"prev hash is not a byte string", stackitem.NewArray(replaceAt(good, 2, stackitem.NewArray(nil))), false},
			{"merkle root is not a byte string", stackitem.NewArray(replaceAt(good, 3, stackitem.NewArray(nil))), false},
			{"timestamp is not a number", stackitem.NewArray(replaceAt(good, 4, stackitem.NewArray(nil))), false},
			{"nonce is not a number", stackitem.NewArray(replaceAt(good, 5, stackitem.NewArray(nil))), false},
			{"index is not a number", stackitem.NewArray(replaceAt(good, 6, stackitem.NewArray(nil))), false},
			{"index is out of range", stackitem.NewArray(replaceAt(good, 6, stackitem.Make(int64(math.MaxUint32)+1))), false},
			{"primary index is not a number", stackitem.NewArray(replaceAt(good, 7, stackitem.NewArray(nil))), false},
			{"primary index is out of range", stackitem.NewArray(replaceAt(good, 7, stackitem.Make(math.MaxUint32))), false},
			{"next consensus is not a byte string", stackitem.NewArray(replaceAt(good, 8, stackitem.NewArray(nil))), false},
			{"transactions length is not a number", stackitem.NewArray(replaceAt(good, 9, stackitem.NewArray(nil))), false},
			{"transactions length is out of range", stackitem.NewArray(replaceAt(good, 9, stackitem.Make(math.MaxUint16+1))), false},
			{"prev state root is not a byte string", stackitem.NewArray(append(append([]stackitem.Item{}, good...), stackitem.NewArray(nil))), true},
		}
	)
	for _, cs := range badCases {
		t.Run(cs.name, func(t *testing.T) {
			b := &Block{StateRootEnabled: cs.stateRootEnabled}
			err := b.FromStackItem(cs.item)
			require.Error(t, err)
		})
	}

	t.Run("good, without state root", func(t *testing.T) {
		b := new(Block)
		require.NoError(t, b.FromStackItem(stackitem.NewArray(good)))
		require.False(t, b.StateRootEnabled)
	})
	t.Run("good, with state root", func(t *testing.T) {
		b := &Block{StateRootEnabled: true}
		require.NoError(t, b.FromStackItem(stackitem.NewArray(append(append([]stackitem.Item{}, good...), prevStateRoot))))
		require.True(t, b.StateRootEnabled)
	})
}

func TestBlock_ToSCParameter(t *testing.T) {
	check := func(t *testing.T, stateRootEnabled bool) {
		b := newTestBlock(stateRootEnabled)

		prm, err := b.ToSCParameter()
		require.NoError(t, err)
		require.Equal(t, smartcontract.ArrayType, prm.Type)
		arr, ok := prm.Value.([]smartcontract.Parameter)
		require.True(t, ok)
		expectedLen := 10
		if stateRootEnabled {
			expectedLen = 11
		}
		require.Len(t, arr, expectedLen)

		require.Equal(t, smartcontract.Parameter{Type: smartcontract.Hash256Type, Value: b.Hash}, arr[0])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(b.Version))}, arr[1])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.Hash256Type, Value: b.PrevHash}, arr[2])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.Hash256Type, Value: b.MerkleRoot}, arr[3])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(b.Timestamp))}, arr[4])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(b.Nonce))}, arr[5])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(b.Index))}, arr[6])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(b.PrimaryIndex))}, arr[7])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.Hash160Type, Value: b.NextConsensus}, arr[8])
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(b.TransactionsLength))}, arr[9])
		if stateRootEnabled {
			require.Equal(t, smartcontract.Parameter{Type: smartcontract.Hash256Type, Value: b.PrevStateRoot}, arr[10])
		}

		t.Run("round trip via stack item", func(t *testing.T) {
			item, err := prm.ToStackItem()
			require.NoError(t, err)

			actual := &Block{StateRootEnabled: stateRootEnabled}
			require.NoError(t, actual.FromStackItem(item))
			require.Equal(t, b, actual)
		})
	}
	t.Run("StateRoot enabled", func(t *testing.T) {
		check(t, true)
	})
	t.Run("StateRoot disabled", func(t *testing.T) {
		check(t, false)
	})

	t.Run("nil", func(t *testing.T) {
		var nilBlock *Block
		prm, err := nilBlock.ToSCParameter()
		require.NoError(t, err)
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.AnyType}, prm)
	})
}

// replaceAt returns a copy of items with the element at index i replaced by v.
func replaceAt(items []stackitem.Item, i int, v stackitem.Item) []stackitem.Item {
	cp := make([]stackitem.Item, len(items))
	copy(cp, items)
	cp[i] = v
	return cp
}
