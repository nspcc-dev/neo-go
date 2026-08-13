package interop_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const countPoints = 8

type benchCase struct {
	name         string
	f            func(*interop.Context) error
	itemsToAdd   []any
	amountToPop  int
	isStepNeeded bool
}

func benchInterop(b *testing.B, ic *interop.Context, bc benchCase, cs *state.Contract) {
	if ic == nil {
		blkChain, _ := chain.NewSingle(b)
		var err error
		ic, err = blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
		if err != nil {
			b.Fatal(err)
		}
		ic.VM.LoadScript(nil)
	}
	if cs != nil {
		err := native.PutContractState(ic.DAO, ic.Chain.NativeManagementID(), cs)
		if err != nil {
			b.Fatal(err)
		}
	}
	ic.VM.SetGasLimit(-1)

	for b.Loop() {
		b.StopTimer()
		for _, item := range bc.itemsToAdd {
			ic.VM.Estack().PushVal(item)
		}
		b.StartTimer()

		if err := bc.f(ic); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		for range bc.amountToPop {
			ic.VM.Estack().Pop()
		}
		if bc.isStepNeeded {
			ic.VM.Step()
		}
		b.StartTimer()
	}
}

// --- System.Contract.* ---

func BenchmarkCreateStandardAccount(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		b.Fatal(err)
	}
	pubBytes := priv.PublicKey().Bytes()

	benchInterop(b, nil, benchCase{
		f:           contract.CreateStandardAccount,
		itemsToAdd:  []any{pubBytes},
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetCallFlags(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           contract.GetCallFlags,
		amountToPop: 1,
	}, nil)
}

func BenchmarkCreateMultisigAccount(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		b.Fatal(err)
	}
	pubBytes := priv.PublicKey().Bytes()

	stepPubKeysCount := vm.MaxStackSize / countPoints
	currPubKeysCount := stepPubKeysCount
	for i := 1; i <= countPoints; i++ {
		pubs := make([]any, currPubKeysCount)
		for j := range pubs {
			pubs[j] = pubBytes
		}
		b.Run(fmt.Sprintf("%d", currPubKeysCount), func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           contract.CreateMultisigAccount,
				itemsToAdd:  []any{pubs, 1},
				amountToPop: 1,
			}, nil)
		})
		currPubKeysCount += stepPubKeysCount
	}
}

func BenchmarkContractCall_NameLen(b *testing.B) {
	stepNameLen := manifest.MaxManifestSize / countPoints
	currNameLen := stepNameLen
	for i := 1; i <= countPoints; i++ {
		name := make([]byte, currNameLen)
		cs := &state.Contract{
			ContractBase: state.ContractBase{
				Hash: random.Uint160(),
				Manifest: manifest.Manifest{
					ABI: manifest.ABI{
						Methods: []manifest.Method{
							{
								Name: string(name),
							},
						},
					},
				},
			},
		}
		b.Run(fmt.Sprintf("%d", currNameLen), func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:          contract.Call,
				itemsToAdd: []any{[]any{}, int32(0), name, cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		currNameLen += stepNameLen
	}
}

func BenchmarkContractCall_ArgsCount(b *testing.B) {
	maxArgsCount := getMaxContractParamCount()
	stepArgsCount := maxArgsCount / countPoints
	currArgsCount := stepArgsCount
	for i := 1; i <= countPoints; i++ {
		args := make([]any, currArgsCount)
		cs := newContractWithParamCount(currArgsCount)
		b.Run(fmt.Sprintf("%d", currArgsCount), func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:          contract.Call,
				itemsToAdd: []any{args, int32(0), "", cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		currArgsCount += stepArgsCount
	}
}

func getMaxContractParamCount() int {
	i := sort.Search(vm.MaxStackSize, func(paramCount int) bool {
		cs := newContractWithParamCount(paramCount)
		d := dao.NewSimple(storage.NewMemoryStore(), false)
		return d.PutStorageConvertible(1, []byte{1}, cs) != nil // true = "уже не помещается"
	})
	return i - 1
}

func newContractWithParamCount(paramCount int) *state.Contract {
	return &state.Contract{
		ContractBase: state.ContractBase{
			Hash: random.Uint160(),
			Manifest: manifest.Manifest{
				ABI: manifest.ABI{
					Methods: []manifest.Method{
						{
							Parameters: make([]manifest.Parameter, paramCount),
						},
					},
				},
			},
		},
	}
}

func getMaxManifestPermissionsCount() int {
	i := sort.Search(manifest.MaxManifestSize, func(n int) bool {
		m := manifest.NewManifest("caller")
		m.Permissions = make([]manifest.Permission, n)
		for j := range n {
			m.Permissions[j] = *manifest.NewPermission(manifest.PermissionHash, random.Uint160())
		}
		data, _ := json.Marshal(m)
		return len(data) > manifest.MaxManifestSize
	})
	return i - 1
}

func BenchmarkContractCall_PermissionsCount(b *testing.B) {
	maxPermissionsCount := getMaxManifestPermissionsCount()
	stepPermissionsCount := maxPermissionsCount / countPoints
	currPermissionsCount := stepPermissionsCount
	for i := 1; i <= countPoints; i++ {
		cs := newContractWithParamCount(0)

		callerManifest := manifest.NewManifest("caller")
		callerManifest.Permissions = make([]manifest.Permission, currPermissionsCount)
		for j := range currPermissionsCount - 1 {
			callerManifest.Permissions[j] = *manifest.NewPermission(manifest.PermissionHash, random.Uint160())
		}
		callerManifest.Permissions[currPermissionsCount-1] = *manifest.NewPermission(manifest.PermissionHash, cs.Hash)

		blkChain, _ := chain.NewSingle(b)
		ic, err := blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
		if err != nil {
			b.Fatal(err)
		}
		callerScript := []byte{byte(opcode.RET)}
		callerNEF, err := nef.NewFile(callerScript)
		if err != nil {
			b.Fatal(err)
		}
		ic.VM.LoadNEFMethod(callerNEF, callerManifest, util.Uint160{}, hash.Hash160(callerScript),
			callflag.All, false, 0, -1, nil, nil, false)

		b.Run(fmt.Sprintf("%d", currPermissionsCount), func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:          contract.Call,
				itemsToAdd: []any{[]any{}, int32(0), "", cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		currPermissionsCount += stepPermissionsCount
	}
}

func getMaxMethodsCount() int {
	i := sort.Search(vm.MaxStackSize, func(methodCount int) bool {
		cs := newContractWithMethodCount(methodCount)
		d := dao.NewSimple(storage.NewMemoryStore(), false)
		return d.PutStorageConvertible(1, []byte{1}, cs) != nil // true = "уже не помещается"
	})
	return i - 1
}

func newContractWithMethodCount(methodCount int) *state.Contract {
	methods := make([]manifest.Method, methodCount)
	for i := range methods {
		methods[i] = manifest.Method{Name: fmt.Sprintf("dummy%d", i)}
	}
	if methodCount > 0 {
		methods[methodCount-1] = manifest.Method{Name: "target"}
	}
	return &state.Contract{
		ContractBase: state.ContractBase{
			Hash: random.Uint160(),
			Manifest: manifest.Manifest{
				ABI: manifest.ABI{
					Methods: methods,
				},
			},
		},
	}
}

func BenchmarkContractCall_MethodsCount(b *testing.B) {
	maxMethodsCount := getMaxMethodsCount()
	stepMethodsCount := maxMethodsCount / countPoints
	currMethodsCount := stepMethodsCount
	for i := 1; i <= countPoints; i++ {
		cs := newContractWithMethodCount(currMethodsCount)
		b.Run(fmt.Sprintf("%d", currMethodsCount), func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:          contract.Call,
				itemsToAdd: []any{[]any{}, int32(0), "target", cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		currMethodsCount += stepMethodsCount
	}
}

func BenchmarkContractCallNative_RefsDelta(b *testing.B) {
	stepArraySize := vm.MaxStackSize / countPoints
	currArraySize := stepArraySize - 2
	for i := 1; i <= countPoints; i++ {
		items := make([]stackitem.Item, 0, currArraySize)
		for range currArraySize {
			items = append(items, stackitem.Null{})
		}
		arr := stackitem.NewArray(items)
		b.Run(fmt.Sprintf("%d", currArraySize), func(b *testing.B) {
			blkChain, _ := chain.NewSingle(b)
			ic, err := blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
			if err != nil {
				b.Fatal(err)
			}
			var stdlib interop.Contract
			for _, n := range ic.Natives {
				if n.Metadata().Name == nativenames.StdLib {
					stdlib = n
					break
				}
			}
			var current config.Hardfork
			for _, hf := range config.Hardforks {
				if !ic.IsHardforkEnabled(hf) {
					break
				}
				current = hf
			}
			hfMD := stdlib.Metadata().HFSpecificContractMD(&current)
			method, _ := hfMD.GetMethod("serialize", 1)

			for b.Loop() {
				b.StopTimer()
				ic.SpawnVM()
				ic.VM.SetGasLimit(-1)
				ic.VM.LoadNEFMethod(&hfMD.NEF, &hfMD.Manifest, util.Uint160{}, hfMD.Hash,
					callflag.All, false, method.MD.Offset, -1, nil, nil, false)
				ic.VM.Estack().PushItem(arr)
				if err := ic.VM.Step(); err != nil {
					b.Fatal(err)
				}

				b.StartTimer()
				if err := ic.VM.Step(); err != nil {
					b.Fatal(err)
				}
				b.StopTimer()

				ic.VM.Estack().Pop()
				b.StartTimer()
			}
		})
		currArraySize += stepArraySize
	}
}

// --- System.Crypto.* ---

func BenchmarkCheckSig(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		b.Fatal(err)
	}

	d := dao.NewSimple(storage.NewMemoryStore(), false)
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), DAO: d}
	tx := transaction.New([]byte{0, 1, 2}, 1)
	ic.Container = tx
	sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
	pub := priv.PublicKey().Bytes()

	ic.SpawnVM()
	benchInterop(b, ic, benchCase{
		f:           crypto.ECDSASecp256r1CheckSig,
		itemsToAdd:  []any{sign, pub},
		amountToPop: 1,
	}, nil)
}

func BenchmarkCheckMultisig(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		b.Fatal(err)
	}

	d := dao.NewSimple(storage.NewMemoryStore(), false)
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), DAO: d}
	tx := transaction.New([]byte{0, 1, 2}, 1)
	ic.Container = tx
	sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
	pub := priv.PublicKey().Bytes()

	ic.SpawnVM()

	stepKeysCount := vm.MaxStackSize / 2 / countPoints
	currKeysCount := stepKeysCount
	for i := 1; i <= countPoints; i++ {
		pubs := make([]any, currKeysCount)
		sigs := make([]any, currKeysCount)
		for j := range pubs {
			pubs[j] = pub
			sigs[j] = sign
		}
		b.Run(fmt.Sprintf("%d", currKeysCount), func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           crypto.ECDSASecp256r1CheckMultisig,
				itemsToAdd:  []any{sigs, pubs},
				amountToPop: 1,
			}, nil)
		})
		currKeysCount += stepKeysCount
	}
}

// --- System.Iterator.* ---

func BenchmarkIteratorNext(b *testing.B) {
	ch := make(chan storage.KeyValue)
	close(ch)
	it := stackitem.NewInterop(istorage.NewIterator(ch, nil, 0))
	benchInterop(b, nil, benchCase{
		f:           iterator.Next,
		itemsToAdd:  []any{it},
		amountToPop: 1,
	}, nil)
}

func benchIteratorValue(b *testing.B, name string, item stackitem.Item) {
	itemBytes, err := stackitem.Serialize(item)
	require.NoError(b, err)
	ch := make(chan storage.KeyValue, 1)
	it := istorage.NewIterator(ch, nil, istorage.FindDeserialize|istorage.FindValuesOnly)

	ch <- storage.KeyValue{
		Key:   []byte{42},
		Value: itemBytes,
	}
	it.Next()

	b.Run(name, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           iterator.Value,
			itemsToAdd:  []any{stackitem.NewInterop(it)},
			amountToPop: 1,
		}, nil)
	})
}

func BenchmarkIteratorValue(b *testing.B) {
	stepArraySize := vm.MaxStackSize / countPoints
	currArraySize := stepArraySize - 1
	for range countPoints {
		benchIteratorValue(b, fmt.Sprintf("refs_delta=%d", currArraySize), stackitem.Make(make([]any, currArraySize)))
		currArraySize += stepArraySize
	}

	stepNumBytes := stackitem.MaxSize / countPoints
	currNumBytes := stepNumBytes
	for range countPoints {
		benchIteratorValue(b, fmt.Sprintf("num_bytes=%d", currNumBytes), stackitem.Make(make([]byte, currNumBytes)))
		currNumBytes += stepNumBytes
	}
}

// --- System.Runtime.* ---

func BenchmarkBurnGas(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.BurnGas,
		itemsToAdd:  []any{int64(1)},
		amountToPop: 0,
	}, nil)
}

func BenchmarkGasLeft(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GasLeft,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetNotifications(b *testing.B) {
	stepNotificationsCount := vm.MaxStackSize / countPoints
	currNotificationsCount := stepNotificationsCount
	ic := &interop.Context{}
	ic.SpawnVM()
	for i := 1; i <= countPoints; i++ {
		notifications := make([]state.NotificationEvent, currNotificationsCount)
		for j := range notifications {
			notifications[j] = state.NotificationEvent{Item: stackitem.NewArray(nil)}
		}
		ic.Notifications = notifications

		b.Run(fmt.Sprintf("%d", currNotificationsCount), func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           runtime.GetNotifications,
				itemsToAdd:  []any{stackitem.Null{}},
				amountToPop: 1,
			}, nil)
		})
		currNotificationsCount += stepNotificationsCount
	}
}

func BenchmarkGetNotifications_RefsDelta(b *testing.B) {
	stepArraySize := vm.MaxStackSize / countPoints
	currArraySize := stepArraySize - 1
	ic := &interop.Context{}
	ic.SpawnVM()
	for i := 1; i <= countPoints; i++ {
		items := make([]stackitem.Item, currArraySize)
		for j := range items {
			items[j] = stackitem.Null{}
		}
		ic.Notifications = []state.NotificationEvent{
			{Item: stackitem.NewArray(items)},
		}

		b.Run(fmt.Sprintf("%d", currArraySize), func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           runtime.GetNotifications,
				itemsToAdd:  []any{stackitem.Null{}},
				amountToPop: 1,
			}, nil)
		})
		currArraySize += stepArraySize
	}
}

func BenchmarkLoadScript_ScriptLen(b *testing.B) {
	stepScriptLen := stackitem.MaxSize / countPoints
	currScriptLen := stepScriptLen
	for i := 1; i <= countPoints; i++ {
		script := make([]byte, currScriptLen)
		for j := range script {
			script[j] = byte(opcode.RET)
		}
		b.Run(fmt.Sprintf("%d", currScriptLen), func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:          runtime.LoadScript,
				itemsToAdd: []any{[]any{}, int(callflag.All), script},
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes its first RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		currScriptLen += stepScriptLen
	}
}

func BenchmarkLoadScript_ArgsCount(b *testing.B) {
	script := []byte{byte(opcode.RET)}
	stepArgsCount := vm.MaxStackSize / countPoints
	currArgsCount := stepArgsCount - 1
	for i := 1; i <= countPoints; i++ {
		args := make([]any, currArgsCount)
		for j := range args {
			args[j] = j
		}
		b.Run(fmt.Sprintf("%d", currArgsCount), func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           runtime.LoadScript,
				itemsToAdd:  []any{args, int(callflag.All), script},
				amountToPop: currArgsCount,
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes the RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		currArgsCount += stepArgsCount
	}
}

func BenchmarkLoadScript_RefsDelta(b *testing.B) {
	script := []byte{byte(opcode.RET)}
	stepArraySize := vm.MaxStackSize / countPoints
	currArraySize := stepArraySize - 1
	for i := 1; i <= countPoints; i++ {
		arg := stackitem.Make(make([]any, currArraySize))
		b.Run(fmt.Sprintf("%d", currArraySize), func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           runtime.LoadScript,
				itemsToAdd:  []any{[]any{arg}, int(callflag.All), script},
				amountToPop: 1,
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes the RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		currArraySize += stepArraySize
	}
}

func BenchmarkGetAddressVersion(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetAddressVersion,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetCallingScriptHash(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetCallingScriptHash,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetEntryScriptHash(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetEntryScriptHash,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetExecutingScriptHash(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetExecutingScriptHash,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetInvocationCounter(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetInvocationCounter,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetNetwork(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetNetwork,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetRandom(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetRandom,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetScriptContainer(b *testing.B) {
	ic := &interop.Context{
		Container: &transaction.Transaction{
			Signers: []transaction.Signer{{}},
		},
	}
	ic.SpawnVM()

	benchInterop(b, ic, benchCase{
		f:           runtime.GetScriptContainer,
		amountToPop: 1,
	}, nil)
}

func getMaxSignersCount() int {
	i := sort.Search(transaction.MaxTransactionSize, func(n int) bool {
		tx := transaction.New(nil, 0)
		tx.Signers = make([]transaction.Signer, n)
		return tx.Size() > transaction.MaxTransactionSize
	})
	return i - 1
}

func BenchmarkCurrentSigners(b *testing.B) {
	maxSignersCount := getMaxSignersCount()
	stepSignersCount := maxSignersCount / countPoints
	currSignersCount := stepSignersCount
	ic := &interop.Context{}
	ic.SpawnVM()
	for i := 1; i <= countPoints; i++ {
		tx := transaction.New(nil, 0)
		tx.Signers = make([]transaction.Signer, currSignersCount)
		ic.Container = tx

		b.Run(fmt.Sprintf("%d", currSignersCount), func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           runtime.CurrentSigners,
				amountToPop: 1,
			}, nil)
		})
		currSignersCount += stepSignersCount
	}
}

func BenchmarkLog(b *testing.B) {
	ic := &interop.Context{Log: zap.NewNop()}
	ic.SpawnVM()
	msg := string(make([]byte, runtime.MaxNotificationSize))

	benchInterop(b, ic, benchCase{
		f:          runtime.Log,
		itemsToAdd: []any{msg},
	}, nil)
}

func getMaxManifestEventsCount() int {
	i := sort.Search(manifest.MaxManifestSize, func(n int) bool {
		m := manifest.NewManifest("caller")
		m.ABI.Events = make([]manifest.Event, n)
		for j := range n {
			m.ABI.Events[j] = manifest.Event{Name: fmt.Sprintf("e%d", j)}
		}
		data, _ := json.Marshal(m)
		return len(data) > manifest.MaxManifestSize
	})
	return i - 1
}

func BenchmarkNotify_EventsCount(b *testing.B) {
	maxEventsCount := getMaxManifestEventsCount()
	stepEventsCount := maxEventsCount / countPoints
	currEventsCount := stepEventsCount
	for i := 1; i <= countPoints; i++ {
		events := make([]manifest.Event, currEventsCount)
		for j := range events {
			events[j] = manifest.Event{Name: fmt.Sprintf("e%d", j)}
		}
		events[currEventsCount-1] = manifest.Event{Name: "target"}
		m := manifest.NewManifest("caller")
		m.ABI.Events = events

		script := []byte{byte(opcode.RET)}
		nefFile, err := nef.NewFile(script)
		if err != nil {
			b.Fatal(err)
		}
		d := dao.NewSimple(storage.NewMemoryStore(), false)
		ic := &interop.Context{DAO: d}
		ic.SpawnVM()
		ic.VM.LoadNEFMethod(nefFile, m, util.Uint160{}, hash.Hash160(script), callflag.All, false, 0, -1, nil, nil, false)

		b.Run(fmt.Sprintf("%d", currEventsCount), func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:          runtime.Notify,
				itemsToAdd: []any{[]any{}, "target"},
			}, nil)
		})
		currEventsCount += stepEventsCount
	}
}

func getMaxNotifyArgsCount() int {
	d := dao.NewSimple(storage.NewMemoryStore(), false)
	i := sort.Search(vm.MaxStackSize, func(n int) bool {
		items := make([]stackitem.Item, n)
		for j := range items {
			items[j] = stackitem.Null{}
		}
		bytes, err := d.GetItemCtx().Serialize(stackitem.NewArray(items), false)
		if err != nil {
			return true
		}
		return len(bytes) > runtime.MaxNotificationSize
	})
	return i - 1
}

func BenchmarkNotify_ParamsCount(b *testing.B) {
	maxParamsCount := getMaxNotifyArgsCount()
	stepParamsCount := maxParamsCount / countPoints
	currParamsCount := stepParamsCount
	for i := 1; i <= countPoints; i++ {
		params := make([]manifest.Parameter, currParamsCount)
		args := make([]any, currParamsCount)
		for j := range params {
			params[j] = manifest.NewParameter(fmt.Sprintf("p%d", j), smartcontract.AnyType)
			args[j] = stackitem.Null{}
		}
		m := manifest.NewManifest("caller")
		m.ABI.Events = []manifest.Event{{Name: "target", Parameters: params}}

		script := []byte{byte(opcode.RET)}
		nefFile, err := nef.NewFile(script)
		if err != nil {
			b.Fatal(err)
		}
		d := dao.NewSimple(storage.NewMemoryStore(), false)
		ic := &interop.Context{DAO: d}
		ic.SpawnVM()
		ic.VM.LoadNEFMethod(nefFile, m, util.Uint160{}, hash.Hash160(script), callflag.All, false, 0, -1, nil, nil, false)

		b.Run(fmt.Sprintf("%d", currParamsCount), func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:          runtime.Notify,
				itemsToAdd: []any{args, "target"},
			}, nil)
		})
		currParamsCount += stepParamsCount
	}
}

func BenchmarkGetTime(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetTime,
		amountToPop: 1,
	}, nil)
}

func BenchmarkGetTrigger(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.GetTrigger,
		amountToPop: 1,
	}, nil)
}

func BenchmarkPlatform(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           runtime.Platform,
		amountToPop: 1,
	}, nil)
}
