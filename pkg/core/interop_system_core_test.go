package core

import (
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

var pathToInternalContracts = filepath.Join("..", "..", "internal", "contracts")

// Tests are taken from
// https://github.com/neo-project/neo/blob/master/tests/neo.UnitTests/SmartContract/UT_ApplicationEngine.Runtime.cs
func TestRuntimeGetRandomCompatibility(t *testing.T) {
	bc := newTestChain(t)

	b := getSharpTestGenesis(t)
	tx := getSharpTestTx(util.Uint160{})
	ic := bc.newInteropContext(trigger.Application, bc.dao.GetWrapped(), b, tx)
	ic.Network = 860833102 // Old mainnet magic used by C# tests.

	ic.VM = vm.New()
	ic.VM.LoadScript([]byte{0x01})
	ic.VM.GasLimit = 1100_00000000

	require.NoError(t, runtime.GetRandom(ic))
	require.Equal(t, "271339657438512451304577787170704246350", ic.VM.Estack().Pop().BigInt().String())

	require.NoError(t, runtime.GetRandom(ic))
	require.Equal(t, "98548189559099075644778613728143131367", ic.VM.Estack().Pop().BigInt().String())

	require.NoError(t, runtime.GetRandom(ic))
	require.Equal(t, "247654688993873392544380234598471205121", ic.VM.Estack().Pop().BigInt().String())

	require.NoError(t, runtime.GetRandom(ic))
	require.Equal(t, "291082758879475329976578097236212073607", ic.VM.Estack().Pop().BigInt().String())

	require.NoError(t, runtime.GetRandom(ic))
	require.Equal(t, "247152297361212656635216876565962360375", ic.VM.Estack().Pop().BigInt().String())
}

func getSharpTestTx(sender util.Uint160) *transaction.Transaction {
	tx := transaction.New([]byte{byte(opcode.PUSH2)}, 0)
	tx.Nonce = 0
	tx.Signers = append(tx.Signers, transaction.Signer{
		Account: sender,
		Scopes:  transaction.CalledByEntry,
	})
	tx.Attributes = []transaction.Attribute{}
	tx.Scripts = append(tx.Scripts, transaction.Witness{InvocationScript: []byte{}, VerificationScript: []byte{}})
	return tx
}

func getSharpTestGenesis(t *testing.T) *block.Block {
	const configPath = "../../config"

	cfg, err := config.Load(configPath, netmode.MainNet)
	require.NoError(t, err)
	b, err := createGenesisBlock(cfg.ProtocolConfiguration)
	require.NoError(t, err)
	return b
}

func TestRuntimeGetNotifications(t *testing.T) {
	v, ic, _ := createVM(t)

	ic.Notifications = []state.NotificationEvent{
		{ScriptHash: util.Uint160{1}, Name: "Event1", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{11})})},
		{ScriptHash: util.Uint160{2}, Name: "Event2", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{22})})},
		{ScriptHash: util.Uint160{1}, Name: "Event1", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{33})})},
	}

	t.Run("NoFilter", func(t *testing.T) {
		v.Estack().PushVal(stackitem.Null{})
		require.NoError(t, runtime.GetNotifications(ic))

		arr := v.Estack().Pop().Array()
		require.Equal(t, len(ic.Notifications), len(arr))
		for i := range arr {
			elem := arr[i].Value().([]stackitem.Item)
			require.Equal(t, ic.Notifications[i].ScriptHash.BytesBE(), elem[0].Value())
			name, err := stackitem.ToString(elem[1])
			require.NoError(t, err)
			require.Equal(t, ic.Notifications[i].Name, name)
			ic.Notifications[i].Item.MarkAsReadOnly() // tiny hack for test to be able to compare object references.
			require.Equal(t, ic.Notifications[i].Item, elem[2])
		}
	})

	t.Run("WithFilter", func(t *testing.T) {
		h := util.Uint160{2}.BytesBE()
		v.Estack().PushVal(h)
		require.NoError(t, runtime.GetNotifications(ic))

		arr := v.Estack().Pop().Array()
		require.Equal(t, 1, len(arr))
		elem := arr[0].Value().([]stackitem.Item)
		require.Equal(t, h, elem[0].Value())
		name, err := stackitem.ToString(elem[1])
		require.NoError(t, err)
		require.Equal(t, ic.Notifications[1].Name, name)
		require.Equal(t, ic.Notifications[1].Item, elem[2])
	})
}

func TestRuntimeGetInvocationCounter(t *testing.T) {
	v, ic, bc := createVM(t)

	cs, _ := contracts.GetTestContractState(t, pathToInternalContracts, 4, 5, random.Uint160()) // sender and IDs are not important for the test
	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, cs))

	ic.Invocations[hash.Hash160([]byte{2})] = 42

	t.Run("No invocations", func(t *testing.T) {
		v.Load([]byte{1})
		// do not return an error in this case.
		require.NoError(t, runtime.GetInvocationCounter(ic))
		require.EqualValues(t, 1, v.Estack().Pop().BigInt().Int64())
	})
	t.Run("NonZero", func(t *testing.T) {
		v.Load([]byte{2})
		require.NoError(t, runtime.GetInvocationCounter(ic))
		require.EqualValues(t, 42, v.Estack().Pop().BigInt().Int64())
	})
	t.Run("Contract", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, cs.Hash, "invocCounter", callflag.All)
		v.LoadWithFlags(w.Bytes(), callflag.All)
		require.NoError(t, v.Run())
		require.EqualValues(t, 1, v.Estack().Pop().BigInt().Int64())
	})
}

func TestStoragePut(t *testing.T) {
	_, cs, ic, bc := createVMAndContractState(t)

	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, cs))

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
	v, cs, ic, bc := createVMAndContractState(t)

	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, cs))
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
			require.NoError(b, chain.contracts.Management.PutContractState(chain.dao, contractState))

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
					require.NoError(b, chain.contracts.Management.PutContractState(chain.dao, contractState))

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

	require.NoError(t, chain.contracts.Management.PutContractState(chain.dao, contractState))

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

func loadScript(ic *interop.Context, script []byte, args ...interface{}) {
	ic.SpawnVM()
	ic.VM.LoadScriptWithFlags(script, callflag.AllowCall)
	for i := range args {
		ic.VM.Estack().PushVal(args[i])
	}
	ic.VM.GasLimit = -1
}

func loadScriptWithHashAndFlags(ic *interop.Context, script []byte, hash util.Uint160, f callflag.CallFlag, args ...interface{}) {
	ic.SpawnVM()
	ic.VM.LoadScriptWithHash(script, hash, f)
	for i := range args {
		ic.VM.Estack().PushVal(args[i])
	}
	ic.VM.GasLimit = -1
}

func TestContractCall(t *testing.T) {
	_, ic, bc := createVM(t)

	cs, currCs := contracts.GetTestContractState(t, pathToInternalContracts, 4, 5, random.Uint160()) // sender and IDs are not important for the test
	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, cs))
	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, currCs))

	currScript := currCs.NEF.Script
	h := cs.Hash

	addArgs := stackitem.NewArray([]stackitem.Item{stackitem.Make(1), stackitem.Make(2)})
	t.Run("Good", func(t *testing.T) {
		t.Run("2 arguments", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(addArgs)
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("add")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, big.NewInt(3), ic.VM.Estack().Pop().Value())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
		t.Run("3 arguments", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(
				append(addArgs.Value().([]stackitem.Item), stackitem.Make(3))))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("add")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, big.NewInt(6), ic.VM.Estack().Pop().Value())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
	})

	t.Run("CallExInvalidFlag", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(addArgs)
		ic.VM.Estack().PushVal(byte(0xFF))
		ic.VM.Estack().PushVal("add")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.Error(t, contract.Call(ic))
	})

	runInvalid := func(args ...interface{}) func(t *testing.T) {
		return func(t *testing.T) {
			loadScriptWithHashAndFlags(ic, currScript, h, callflag.All, 42)
			for i := range args {
				ic.VM.Estack().PushVal(args[i])
			}
			// interops can both return error and panic,
			// we don't care which kind of error has occurred
			require.Panics(t, func() {
				err := contract.Call(ic)
				if err != nil {
					panic(err)
				}
			})
		}
	}

	t.Run("Invalid", func(t *testing.T) {
		t.Run("Hash", runInvalid(addArgs, "add", h.BytesBE()[1:]))
		t.Run("MissingHash", runInvalid(addArgs, "add", util.Uint160{}.BytesBE()))
		t.Run("Method", runInvalid(addArgs, stackitem.NewInterop("add"), h.BytesBE()))
		t.Run("MissingMethod", runInvalid(addArgs, "sub", h.BytesBE()))
		t.Run("DisallowedMethod", runInvalid(stackitem.NewArray(nil), "ret7", h.BytesBE()))
		t.Run("Arguments", runInvalid(1, "add", h.BytesBE()))
		t.Run("NotEnoughArguments", runInvalid(
			stackitem.NewArray([]stackitem.Item{stackitem.Make(1)}), "add", h.BytesBE()))
		t.Run("TooMuchArguments", runInvalid(
			stackitem.NewArray([]stackitem.Item{
				stackitem.Make(1), stackitem.Make(2), stackitem.Make(3), stackitem.Make(4)}),
			"add", h.BytesBE()))
	})

	t.Run("ReturnValues", func(t *testing.T) {
		t.Run("Many", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(nil))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("invalidReturn")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.Error(t, ic.VM.Run())
		})
		t.Run("Void", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(nil))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("justReturn")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, stackitem.Null{}, ic.VM.Estack().Pop().Item())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
	})

	t.Run("IsolatedStack", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(stackitem.NewArray(nil))
		ic.VM.Estack().PushVal(callflag.All)
		ic.VM.Estack().PushVal("drop")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.Error(t, ic.VM.Run())
	})

	t.Run("CallInitialize", func(t *testing.T) {
		t.Run("Directly", runInvalid(stackitem.NewArray([]stackitem.Item{}), "_initialize", h.BytesBE()))

		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.Make(5)}))
		ic.VM.Estack().PushVal(callflag.All)
		ic.VM.Estack().PushVal("add3")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.NoError(t, ic.VM.Run())
		require.Equal(t, 2, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(8), ic.VM.Estack().Pop().Value())
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
	})
}

func TestContractGetCallFlags(t *testing.T) {
	v, ic, _ := createVM(t)

	v.LoadScriptWithHash([]byte{byte(opcode.RET)}, util.Uint160{1, 2, 3}, callflag.All)
	require.NoError(t, contractGetCallFlags(ic))
	require.Equal(t, int64(callflag.All), v.Estack().Pop().Value().(*big.Int).Int64())
}

func TestRuntimeCheckWitness(t *testing.T) {
	_, ic, bc := createVM(t)

	script := []byte{byte(opcode.RET)}
	scriptHash := hash.Hash160(script)
	check := func(t *testing.T, ic *interop.Context, arg interface{}, shouldFail bool, expected ...bool) {
		ic.VM.Estack().PushVal(arg)
		err := runtime.CheckWitness(ic)
		if shouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, expected)
			actual, ok := ic.VM.Estack().Pop().Value().(bool)
			require.True(t, ok)
			require.Equal(t, expected[0], actual)
		}
	}
	t.Run("error", func(t *testing.T) {
		t.Run("not a hash or key", func(t *testing.T) {
			check(t, ic, []byte{1, 2, 3}, true)
		})
		t.Run("script container is not a transaction", func(t *testing.T) {
			loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
			check(t, ic, random.Uint160().BytesBE(), true)
		})
		t.Run("check scope", func(t *testing.T) {
			t.Run("CustomGroups, missing ReadStates flag", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account:       hash,
							Scopes:        transaction.CustomGroups,
							AllowedGroups: []*keys.PublicKey{},
						},
					},
				}
				ic.Tx = tx
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, callflag.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.AllowCall)
				check(t, ic, hash.BytesBE(), true)
			})
			t.Run("Rules, missing ReadStates flag", func(t *testing.T) {
				hash := random.Uint160()
				pk, err := keys.NewPrivateKey()
				require.NoError(t, err)
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.Rules,
							Rules: []transaction.WitnessRule{{
								Action:    transaction.WitnessAllow,
								Condition: (*transaction.ConditionGroup)(pk.PublicKey()),
							}},
						},
					},
				}
				ic.Tx = tx
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, callflag.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.AllowCall)
				check(t, ic, hash.BytesBE(), true)
			})
		})
	})
	t.Run("positive", func(t *testing.T) {
		t.Run("calling scripthash", func(t *testing.T) {
			t.Run("hashed witness", func(t *testing.T) {
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, callflag.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.All)
				check(t, ic, callingScriptHash.BytesBE(), false, true)
			})
			t.Run("keyed witness", func(t *testing.T) {
				pk, err := keys.NewPrivateKey()
				require.NoError(t, err)
				callingScriptHash := pk.PublicKey().GetScriptHash()
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, callflag.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.All)
				check(t, ic, pk.PublicKey().Bytes(), false, true)
			})
		})
		t.Run("check scope", func(t *testing.T) {
			t.Run("Global", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.Global,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
				ic.Tx = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CalledByEntry", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.CalledByEntry,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
				ic.Tx = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CustomContracts", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account:          hash,
							Scopes:           transaction.CustomContracts,
							AllowedContracts: []util.Uint160{scriptHash},
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
				ic.Tx = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CustomGroups", func(t *testing.T) {
				t.Run("unknown scripthash", func(t *testing.T) {
					hash := random.Uint160()
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account:       hash,
								Scopes:        transaction.CustomGroups,
								AllowedGroups: []*keys.PublicKey{},
							},
						},
					}
					loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
					ic.Tx = tx
					check(t, ic, hash.BytesBE(), false, false)
				})
				t.Run("positive", func(t *testing.T) {
					targetHash := random.Uint160()
					pk, err := keys.NewPrivateKey()
					require.NoError(t, err)
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account:       targetHash,
								Scopes:        transaction.CustomGroups,
								AllowedGroups: []*keys.PublicKey{pk.PublicKey()},
							},
						},
					}
					contractScript := []byte{byte(opcode.PUSH1), byte(opcode.RET)}
					contractScriptHash := hash.Hash160(contractScript)
					ne, err := nef.NewFile(contractScript)
					require.NoError(t, err)
					contractState := &state.Contract{
						ContractBase: state.ContractBase{
							ID:   15,
							Hash: contractScriptHash,
							NEF:  *ne,
							Manifest: manifest.Manifest{
								Groups: []manifest.Group{{PublicKey: pk.PublicKey(), Signature: make([]byte, keys.SignatureLen)}},
							},
						},
					}
					require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, contractState))
					loadScriptWithHashAndFlags(ic, contractScript, contractScriptHash, callflag.All)
					ic.Tx = tx
					check(t, ic, targetHash.BytesBE(), false, true)
				})
			})
			t.Run("Rules", func(t *testing.T) {
				t.Run("no match", func(t *testing.T) {
					hash := random.Uint160()
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account: hash,
								Scopes:  transaction.Rules,
								Rules: []transaction.WitnessRule{{
									Action:    transaction.WitnessAllow,
									Condition: (*transaction.ConditionScriptHash)(&hash),
								}},
							},
						},
					}
					loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
					ic.Tx = tx
					check(t, ic, hash.BytesBE(), false, false)
				})
				t.Run("allow", func(t *testing.T) {
					hash := random.Uint160()
					var cond = true
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account: hash,
								Scopes:  transaction.Rules,
								Rules: []transaction.WitnessRule{{
									Action:    transaction.WitnessAllow,
									Condition: (*transaction.ConditionBoolean)(&cond),
								}},
							},
						},
					}
					loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
					ic.Tx = tx
					check(t, ic, hash.BytesBE(), false, true)
				})
				t.Run("deny", func(t *testing.T) {
					hash := random.Uint160()
					var cond = true
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account: hash,
								Scopes:  transaction.Rules,
								Rules: []transaction.WitnessRule{{
									Action:    transaction.WitnessDeny,
									Condition: (*transaction.ConditionBoolean)(&cond),
								}},
							},
						},
					}
					loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
					ic.Tx = tx
					check(t, ic, hash.BytesBE(), false, false)
				})
			})
			t.Run("bad scope", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.None,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
				ic.Tx = tx
				check(t, ic, hash.BytesBE(), false, false)
			})
		})
	})
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
