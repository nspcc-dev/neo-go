package core

import (
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

var pathToInternalContracts = filepath.Join("..", "..", "internal", "contracts")

func TestStoragePut(t *testing.T) {
	_, cs, ic, _ := createVMAndContractState(t)

	require.NoError(t, native.PutContractState(ic.DAO, cs))

	initVM := func(t *testing.T, key, value []byte, gas int64) {
		v := ic.SpawnVM()
		v.LoadScript(cs.NEF.Script)
		v.GasLimit = gas
		v.Estack().PushVal(value)
		v.Estack().PushVal(key)
		require.NoError(t, storageGetContext(ic))
	}

	t.Run("create, not enough gas", func(t *testing.T) {
		initVM(t, []byte{1}, []byte{2, 3}, 2*native.DefaultStoragePrice)
		err := storagePut(ic)
		require.True(t, errors.Is(err, errGasLimitExceeded), "got: %v", err)
	})

	initVM(t, []byte{4}, []byte{5, 6}, 3*native.DefaultStoragePrice)
	require.NoError(t, storagePut(ic))

	t.Run("update", func(t *testing.T) {
		t.Run("not enough gas", func(t *testing.T) {
			initVM(t, []byte{4}, []byte{5, 6, 7, 8}, native.DefaultStoragePrice)
			err := storagePut(ic)
			require.True(t, errors.Is(err, errGasLimitExceeded), "got: %v", err)
		})
		initVM(t, []byte{4}, []byte{5, 6, 7, 8}, 3*native.DefaultStoragePrice)
		require.NoError(t, storagePut(ic))
		initVM(t, []byte{4}, []byte{5, 6}, native.DefaultStoragePrice)
		require.NoError(t, storagePut(ic))
	})

	t.Run("check limits", func(t *testing.T) {
		initVM(t, make([]byte, storage.MaxStorageKeyLen), make([]byte, storage.MaxStorageValueLen), -1)
		require.NoError(t, storagePut(ic))
	})

	t.Run("bad", func(t *testing.T) {
		t.Run("readonly context", func(t *testing.T) {
			initVM(t, []byte{1}, []byte{1}, -1)
			require.NoError(t, storageContextAsReadOnly(ic))
			require.Error(t, storagePut(ic))
		})
		t.Run("big key", func(t *testing.T) {
			initVM(t, make([]byte, storage.MaxStorageKeyLen+1), []byte{1}, -1)
			require.Error(t, storagePut(ic))
		})
		t.Run("big value", func(t *testing.T) {
			initVM(t, []byte{1}, make([]byte, storage.MaxStorageValueLen+1), -1)
			require.Error(t, storagePut(ic))
		})
	})
}

func TestStorageDelete(t *testing.T) {
	v, cs, ic, _ := createVMAndContractState(t)

	require.NoError(t, native.PutContractState(ic.DAO, cs))
	v.LoadScriptWithHash(cs.NEF.Script, cs.Hash, callflag.All)
	put := func(key, value string, flag int) {
		v.Estack().PushVal(value)
		v.Estack().PushVal(key)
		require.NoError(t, storageGetContext(ic))
		require.NoError(t, storagePut(ic))
	}
	put("key1", "value1", 0)
	put("key2", "value2", 0)
	put("key3", "value3", 0)

	t.Run("good", func(t *testing.T) {
		v.Estack().PushVal("key1")
		require.NoError(t, storageGetContext(ic))
		require.NoError(t, storageDelete(ic))
	})
	t.Run("readonly context", func(t *testing.T) {
		v.Estack().PushVal("key2")
		require.NoError(t, storageGetReadOnlyContext(ic))
		require.Error(t, storageDelete(ic))
	})
	t.Run("readonly context (from normal)", func(t *testing.T) {
		v.Estack().PushVal("key3")
		require.NoError(t, storageGetContext(ic))
		require.NoError(t, storageContextAsReadOnly(ic))
		require.Error(t, storageDelete(ic))
	})
}

func BenchmarkStorageFind(b *testing.B) {
	for count := 10; count <= 10000; count *= 10 {
		b.Run(fmt.Sprintf("%dElements", count), func(b *testing.B) {
			v, contractState, context, chain := createVMAndContractState(b)
			require.NoError(b, native.PutContractState(chain.dao, contractState))

			items := make(map[string]state.StorageItem)
			for i := 0; i < count; i++ {
				items["abc"+random.String(10)] = random.Bytes(10)
			}
			for k, v := range items {
				context.DAO.PutStorageItem(contractState.ID, []byte(k), v)
				context.DAO.PutStorageItem(contractState.ID+1, []byte(k), v)
			}
			changes, err := context.DAO.Persist()
			require.NoError(b, err)
			require.NotEqual(b, 0, changes)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				v.Estack().PushVal(istorage.FindDefault)
				v.Estack().PushVal("abc")
				v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: contractState.ID}))
				b.StartTimer()
				err := storageFind(context)
				if err != nil {
					b.FailNow()
				}
				b.StopTimer()
				context.Finalize()
			}
		})
	}
}

func BenchmarkStorageFindIteratorNext(b *testing.B) {
	for count := 10; count <= 10000; count *= 10 {
		cases := map[string]int{
			"Pick1":    1,
			"PickHalf": count / 2,
			"PickAll":  count,
		}
		b.Run(fmt.Sprintf("%dElements", count), func(b *testing.B) {
			for name, last := range cases {
				b.Run(name, func(b *testing.B) {
					v, contractState, context, chain := createVMAndContractState(b)
					require.NoError(b, native.PutContractState(chain.dao, contractState))

					items := make(map[string]state.StorageItem)
					for i := 0; i < count; i++ {
						items["abc"+random.String(10)] = random.Bytes(10)
					}
					for k, v := range items {
						context.DAO.PutStorageItem(contractState.ID, []byte(k), v)
						context.DAO.PutStorageItem(contractState.ID+1, []byte(k), v)
					}
					changes, err := context.DAO.Persist()
					require.NoError(b, err)
					require.NotEqual(b, 0, changes)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						b.StopTimer()
						v.Estack().PushVal(istorage.FindDefault)
						v.Estack().PushVal("abc")
						v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: contractState.ID}))
						b.StartTimer()
						err := storageFind(context)
						b.StopTimer()
						if err != nil {
							b.FailNow()
						}
						res := context.VM.Estack().Pop().Item()
						for i := 0; i < last; i++ {
							context.VM.Estack().PushVal(res)
							b.StartTimer()
							require.NoError(b, iterator.Next(context))
							b.StopTimer()
							require.True(b, context.VM.Estack().Pop().Bool())
						}

						context.VM.Estack().PushVal(res)
						require.NoError(b, iterator.Next(context))
						actual := context.VM.Estack().Pop().Bool()
						if last == count {
							require.False(b, actual)
						} else {
							require.True(b, actual)
						}
						context.Finalize()
					}
				})
			}
		})
	}
}

func TestStorageFind(t *testing.T) {
	v, contractState, context, chain := createVMAndContractState(t)

	arr := []stackitem.Item{
		stackitem.NewBigInteger(big.NewInt(42)),
		stackitem.NewByteArray([]byte("second")),
		stackitem.Null{},
	}
	rawArr, err := stackitem.Serialize(stackitem.NewArray(arr))
	require.NoError(t, err)
	rawArr0, err := stackitem.Serialize(stackitem.NewArray(arr[:0]))
	require.NoError(t, err)
	rawArr1, err := stackitem.Serialize(stackitem.NewArray(arr[:1]))
	require.NoError(t, err)

	skeys := [][]byte{{0x01, 0x02}, {0x02, 0x01}, {0x01, 0x01},
		{0x04, 0x00}, {0x05, 0x00}, {0x06}, {0x07}, {0x08},
		{0x09, 0x12, 0x34}, {0x09, 0x12, 0x56},
	}
	items := []state.StorageItem{
		[]byte{0x01, 0x02, 0x03, 0x04},
		[]byte{0x04, 0x03, 0x02, 0x01},
		[]byte{0x03, 0x04, 0x05, 0x06},
		[]byte{byte(stackitem.ByteArrayT), 2, 0xCA, 0xFE},
		[]byte{0xFF, 0xFF},
		rawArr,
		rawArr0,
		rawArr1,
		[]byte{111},
		[]byte{222},
	}

	require.NoError(t, native.PutContractState(chain.dao, contractState))

	id := contractState.ID

	for i := range skeys {
		context.DAO.PutStorageItem(id, skeys[i], items[i])
	}

	testFind := func(t *testing.T, prefix []byte, opts int64, expected []stackitem.Item) {
		v.Estack().PushVal(opts)
		v.Estack().PushVal(prefix)
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: id}))

		err := storageFind(context)
		require.NoError(t, err)

		var iter *stackitem.Interop
		require.NotPanics(t, func() { iter = v.Estack().Pop().Interop() })

		for i := range expected { // sorted indices with mathing prefix
			v.Estack().PushVal(iter)
			require.NoError(t, iterator.Next(context))
			require.True(t, v.Estack().Pop().Bool())

			v.Estack().PushVal(iter)
			if expected[i] == nil {
				require.Panics(t, func() { _ = iterator.Value(context) })
				return
			}
			require.NoError(t, iterator.Value(context))
			require.Equal(t, expected[i], v.Estack().Pop().Item())
		}

		v.Estack().PushVal(iter)
		require.NoError(t, iterator.Next(context))
		require.False(t, v.Estack().Pop().Bool())
	}

	t.Run("normal invocation", func(t *testing.T) {
		testFind(t, []byte{0x01}, istorage.FindDefault, []stackitem.Item{
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewByteArray(skeys[2]),
				stackitem.NewByteArray(items[2]),
			}),
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewByteArray(skeys[0]),
				stackitem.NewByteArray(items[0]),
			}),
		})
	})

	t.Run("keys only", func(t *testing.T) {
		testFind(t, []byte{0x01}, istorage.FindKeysOnly, []stackitem.Item{
			stackitem.NewByteArray(skeys[2]),
			stackitem.NewByteArray(skeys[0]),
		})
	})
	t.Run("remove prefix", func(t *testing.T) {
		testFind(t, []byte{0x01}, istorage.FindKeysOnly|istorage.FindRemovePrefix, []stackitem.Item{
			stackitem.NewByteArray(skeys[2][1:]),
			stackitem.NewByteArray(skeys[0][1:]),
		})
		testFind(t, []byte{0x09, 0x12}, istorage.FindKeysOnly|istorage.FindRemovePrefix, []stackitem.Item{
			stackitem.NewByteArray(skeys[8][2:]),
			stackitem.NewByteArray(skeys[9][2:]),
		})
	})
	t.Run("values only", func(t *testing.T) {
		testFind(t, []byte{0x01}, istorage.FindValuesOnly, []stackitem.Item{
			stackitem.NewByteArray(items[2]),
			stackitem.NewByteArray(items[0]),
		})
	})
	t.Run("deserialize values", func(t *testing.T) {
		testFind(t, []byte{0x04}, istorage.FindValuesOnly|istorage.FindDeserialize, []stackitem.Item{
			stackitem.NewByteArray(items[3][2:]),
		})
		t.Run("invalid", func(t *testing.T) {
			v.Estack().PushVal(istorage.FindDeserialize)
			v.Estack().PushVal([]byte{0x05})
			v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: id}))
			err := storageFind(context)
			require.NoError(t, err)

			var iter *stackitem.Interop
			require.NotPanics(t, func() { iter = v.Estack().Pop().Interop() })

			v.Estack().PushVal(iter)
			require.NoError(t, iterator.Next(context))

			v.Estack().PushVal(iter)
			require.Panics(t, func() { _ = iterator.Value(context) })
		})
	})
	t.Run("PickN", func(t *testing.T) {
		testFind(t, []byte{0x06}, istorage.FindPick0|istorage.FindValuesOnly|istorage.FindDeserialize, arr[:1])
		testFind(t, []byte{0x06}, istorage.FindPick1|istorage.FindValuesOnly|istorage.FindDeserialize, arr[1:2])
		// Array with 0 elements.
		testFind(t, []byte{0x07}, istorage.FindPick0|istorage.FindValuesOnly|istorage.FindDeserialize,
			[]stackitem.Item{nil})
		// Array with 1 element.
		testFind(t, []byte{0x08}, istorage.FindPick1|istorage.FindValuesOnly|istorage.FindDeserialize,
			[]stackitem.Item{nil})
		// Not an array, but serialized ByteArray.
		testFind(t, []byte{0x04}, istorage.FindPick1|istorage.FindValuesOnly|istorage.FindDeserialize,
			[]stackitem.Item{nil})
	})

	t.Run("normal invocation, empty result", func(t *testing.T) {
		testFind(t, []byte{0x03}, istorage.FindDefault, nil)
	})

	t.Run("invalid options", func(t *testing.T) {
		invalid := []int64{
			istorage.FindKeysOnly | istorage.FindValuesOnly,
			^istorage.FindAll,
			istorage.FindKeysOnly | istorage.FindDeserialize,
			istorage.FindPick0,
			istorage.FindPick0 | istorage.FindPick1 | istorage.FindDeserialize,
			istorage.FindPick0 | istorage.FindPick1,
		}
		for _, opts := range invalid {
			v.Estack().PushVal(opts)
			v.Estack().PushVal([]byte{0x01})
			v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: id}))
			require.Error(t, storageFind(context))
		}
	})
	t.Run("invalid type for StorageContext", func(t *testing.T) {
		v.Estack().PushVal(istorage.FindDefault)
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(nil))

		require.Error(t, storageFind(context))
	})

	t.Run("invalid id", func(t *testing.T) {
		invalidID := id + 1

		v.Estack().PushVal(istorage.FindDefault)
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: invalidID}))

		require.NoError(t, storageFind(context))
		require.NoError(t, iterator.Next(context))
		require.False(t, v.Estack().Pop().Bool())
	})
}

// Helper functions to create VM, InteropContext, TX, Account, Contract.

func createVM(t testing.TB) (*vm.VM, *interop.Context, *Blockchain) {
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application,
		chain.dao.GetWrapped(), nil, nil)
	v := context.SpawnVM()
	return v, context, chain
}

func createVMAndContractState(t testing.TB) (*vm.VM, *state.Contract, *interop.Context, *Blockchain) {
	script := []byte("testscript")
	m := manifest.NewManifest("Test")
	ne, err := nef.NewFile(script)
	require.NoError(t, err)
	contractState := &state.Contract{
		ContractBase: state.ContractBase{
			NEF:      *ne,
			Hash:     hash.Hash160(script),
			Manifest: *m,
			ID:       123,
		},
	}

	v, context, chain := createVM(t)
	return v, contractState, context, chain
}

// TestNativeGetMethod is needed to ensure that methods list has the same sorting
// rule as we expect inside the `ContractMD.GetMethod`.
func TestNativeGetMethod(t *testing.T) {
	cfg := config.ProtocolConfiguration{P2PSigExtensions: true}
	cs := native.NewContracts(cfg)
	for _, c := range cs.Contracts {
		t.Run(c.Metadata().Name, func(t *testing.T) {
			for _, m := range c.Metadata().Methods {
				_, ok := c.Metadata().GetMethod(m.MD.Name, len(m.MD.Parameters))
				require.True(t, ok)
			}
		})
	}
}
