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

func newTestTransaction() *Transaction {
	return &Transaction{
		Hash:            random.Uint256(),
		Version:         0,
		Nonce:           123,
		Sender:          random.Uint160(),
		SystemFee:       100500,
		NetworkFee:      42,
		ValidUntilBlock: 1000,
		Script:          []byte{1, 2, 3},
	}
}

func TestTransaction_ToFromStackItem(t *testing.T) {
	tx := newTestTransaction()
	actual := new(Transaction)
	testserdes.ToFromStackItem(t, tx, actual)
}

func TestTransactionFromStackItem(t *testing.T) {
	var (
		hash    = stackitem.NewByteArray(random.Uint256().BytesBE())
		version = stackitem.Make(0)
		nonce   = stackitem.Make(123)
		sender  = stackitem.NewByteArray(random.Uint160().BytesBE())
		sysFee  = stackitem.Make(100500)
		netFee  = stackitem.Make(42)
		vub     = stackitem.Make(1000)
		script  = stackitem.NewByteArray([]byte{1, 2, 3})

		good = []stackitem.Item{hash, version, nonce, sender, sysFee, netFee, vub, script}

		badCases = []struct {
			name string
			item stackitem.Item
		}{
			{"not an array", stackitem.Make(1)},
			{"wrong number of elements", stackitem.NewArray(good[:7])},
			{"hash is not a byte string", stackitem.NewArray(replaceAt(good, 0, stackitem.NewArray(nil)))},
			{"hash is not a hash", stackitem.NewArray(replaceAt(good, 0, stackitem.NewByteArray([]byte{1, 2, 3})))},
			{"version is not a number", stackitem.NewArray(replaceAt(good, 1, stackitem.NewArray(nil)))},
			{"version is out of range", stackitem.NewArray(replaceAt(good, 1, stackitem.Make(-1)))},
			{"nonce is not a number", stackitem.NewArray(replaceAt(good, 2, stackitem.NewArray(nil)))},
			{"nonce is out of range", stackitem.NewArray(replaceAt(good, 2, stackitem.Make(int64(math.MaxUint32)+1)))},
			{"sender is not a byte string", stackitem.NewArray(replaceAt(good, 3, stackitem.NewArray(nil)))},
			{"system fee is not a number", stackitem.NewArray(replaceAt(good, 4, stackitem.NewArray(nil)))},
			{"network fee is not a number", stackitem.NewArray(replaceAt(good, 5, stackitem.NewArray(nil)))},
			{"valid until block is not a number", stackitem.NewArray(replaceAt(good, 6, stackitem.NewArray(nil)))},
			{"valid until block is out of range", stackitem.NewArray(replaceAt(good, 6, stackitem.Make(int64(math.MaxUint32)+1)))},
			{"script is not a byte string", stackitem.NewArray(replaceAt(good, 7, stackitem.NewArray(nil)))},
		}
	)
	for _, cs := range badCases {
		t.Run(cs.name, func(t *testing.T) {
			tx := new(Transaction)
			err := tx.FromStackItem(cs.item)
			require.Error(t, err)
		})
	}

	t.Run("good", func(t *testing.T) {
		tx := new(Transaction)
		require.NoError(t, tx.FromStackItem(stackitem.NewArray(good)))
	})
}

func TestTransaction_ToSCParameter(t *testing.T) {
	tx := newTestTransaction()

	prm, err := tx.ToSCParameter()
	require.NoError(t, err)
	require.Equal(t, smartcontract.ArrayType, prm.Type)
	arr, ok := prm.Value.([]smartcontract.Parameter)
	require.True(t, ok)
	require.Len(t, arr, 8)

	require.Equal(t, smartcontract.Parameter{Type: smartcontract.Hash256Type, Value: tx.Hash}, arr[0])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(tx.Version))}, arr[1])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(tx.Nonce))}, arr[2])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.Hash160Type, Value: tx.Sender}, arr[3])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(tx.SystemFee)}, arr[4])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(tx.NetworkFee)}, arr[5])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(tx.ValidUntilBlock))}, arr[6])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.ByteArrayType, Value: tx.Script}, arr[7])

	t.Run("round trip via stack item", func(t *testing.T) {
		item, err := prm.ToStackItem()
		require.NoError(t, err)

		actual := new(Transaction)
		require.NoError(t, actual.FromStackItem(item))
		require.Equal(t, tx, actual)
	})

	t.Run("nil", func(t *testing.T) {
		var nilTx *Transaction
		prm, err := nilTx.ToSCParameter()
		require.NoError(t, err)
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.AnyType}, prm)
	})
}
