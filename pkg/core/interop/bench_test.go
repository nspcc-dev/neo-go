package interop_test

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

func genCreateStandardAccount(nopNs float64) float64 {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		panic(err)
	}
	pubBytes := priv.PublicKey().Bytes()

	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           contract.CreateStandardAccount,
			itemsToAdd:  []any{pubBytes},
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetCallFlags(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           contract.GetCallFlags,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genCreateMultisigAccount(nopNs float64) [][]string {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		panic(err)
	}
	pubBytes := priv.PublicKey().Bytes()

	measure := func(n int) []string {
		pubs := make([]any, n)
		for j := range pubs {
			pubs[j] = pubBytes
		}
		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           contract.CreateMultisigAccount,
				itemsToAdd:  []any{pubs, 1},
				amountToPop: 1,
			}, nil)
		})
		return []string{strconv.Itoa(n), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepPubKeysCount := vm.MaxStackSize / countPoints
	currPubKeysCount := stepPubKeysCount
	rows := make([][]string, 0, countPoints+1)
	rows = append(rows, measure(1))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currPubKeysCount))
		currPubKeysCount += stepPubKeysCount
	}
	return rows
}

func genContractCallNameLen(nopNs float64) [][]string {
	measure := func(nameLen int) []string {
		name := make([]byte, nameLen)
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
		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:          contract.Call,
				itemsToAdd: []any{[]any{}, int32(0), name, cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		return []string{strconv.Itoa(nameLen), "0", "0", "1", strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepNameLen := manifest.MaxManifestSize / countPoints
	currNameLen := stepNameLen
	rows := make([][]string, 0, countPoints+1)
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currNameLen))
		currNameLen += stepNameLen
	}
	return rows
}

func genContractCallArgsCount(nopNs float64) [][]string {
	measure := func(argsCount int) []string {
		args := make([]any, argsCount)
		cs := newContractWithParamCount(argsCount)
		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:          contract.Call,
				itemsToAdd: []any{args, int32(0), "", cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		return []string{"0", strconv.Itoa(argsCount), "0", "1", strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	maxArgsCount := getMaxContractParamCount()
	stepArgsCount := maxArgsCount / countPoints
	currArgsCount := stepArgsCount
	rows := make([][]string, 0, countPoints+1)
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currArgsCount))
		currArgsCount += stepArgsCount
	}
	return rows
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

func genContractCallPermissionsCount(nopNs float64) [][]string {
	measure := func(permissionsCount int) []string {
		cs := newContractWithParamCount(0)

		callerManifest := manifest.NewManifest("caller")
		callerManifest.Permissions = make([]manifest.Permission, permissionsCount)
		for j := range permissionsCount - 1 {
			callerManifest.Permissions[j] = *manifest.NewPermission(manifest.PermissionHash, random.Uint160())
		}
		callerManifest.Permissions[permissionsCount-1] = *manifest.NewPermission(manifest.PermissionHash, cs.Hash)

		ns := measureNsPerOp(func(b *testing.B) {
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

			benchInterop(b, ic, benchCase{
				f:          contract.Call,
				itemsToAdd: []any{[]any{}, int32(0), "", cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		return []string{"0", "0", strconv.Itoa(permissionsCount), "1", strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	maxPermissionsCount := getMaxManifestPermissionsCount()
	stepPermissionsCount := maxPermissionsCount / countPoints
	currPermissionsCount := stepPermissionsCount
	rows := make([][]string, 0, countPoints+1)
	rows = append(rows, measure(1))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currPermissionsCount))
		currPermissionsCount += stepPermissionsCount
	}
	return rows
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
		methods[i] = manifest.Method{Name: strconv.Itoa(i)}
	}
	if methodCount > 0 {
		methods[methodCount-1] = manifest.Method{Name: ""}
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

func genContractCallMethodsCount(nopNs float64) [][]string {
	measure := func(methodsCount int) []string {
		cs := newContractWithMethodCount(methodsCount)
		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:          contract.Call,
				itemsToAdd: []any{[]any{}, int32(0), "", cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		// name_len=0 (called name is ""), args_count=0, permissions_count=0.
		return []string{"0", "0", "0", strconv.Itoa(methodsCount), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	maxMethodsCount := getMaxMethodsCount()
	stepMethodsCount := maxMethodsCount / countPoints
	currMethodsCount := stepMethodsCount
	rows := make([][]string, 0, countPoints+1)
	rows = append(rows, measure(1))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currMethodsCount))
		currMethodsCount += stepMethodsCount
	}
	return rows
}

func genContractCallNativeRefsDelta(nopNs float64) [][]string {
	measure := func(arraySize int) []string {
		items := make([]stackitem.Item, 0, arraySize)
		for range arraySize {
			items = append(items, stackitem.Null{})
		}
		arr := stackitem.NewArray(items)
		ns := measureNsPerOp(func(b *testing.B) {
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
		return []string{strconv.Itoa(arraySize + 1), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepArraySize := vm.MaxStackSize / countPoints
	currArraySize := stepArraySize - 2
	rows := make([][]string, 0, countPoints+1)
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currArraySize))
		currArraySize += stepArraySize
	}
	return rows
}

// --- System.Crypto.* ---

func genCheckSig(nopNs float64) float64 {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		panic(err)
	}

	d := dao.NewSimple(storage.NewMemoryStore(), false)
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), DAO: d}
	tx := transaction.New([]byte{0, 1, 2}, 1)
	ic.Container = tx
	sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
	pub := priv.PublicKey().Bytes()

	ic.SpawnVM()
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, ic, benchCase{
			f:           crypto.ECDSASecp256r1CheckSig,
			itemsToAdd:  []any{sign, pub},
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

// --- System.Iterator.* ---

func genIteratorNext(nopNs float64) float64 {
	ch := make(chan storage.KeyValue)
	close(ch)
	it := stackitem.NewInterop(istorage.NewIterator(ch, nil, 0))
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           iterator.Next,
			itemsToAdd:  []any{it},
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func iteratorValueNsPerOp(item stackitem.Item) float64 {
	itemBytes, err := stackitem.Serialize(item)
	if err != nil {
		panic(err)
	}
	ch := make(chan storage.KeyValue, 1)
	it := istorage.NewIterator(ch, nil, istorage.FindDeserialize|istorage.FindValuesOnly)

	ch <- storage.KeyValue{
		Key:   []byte{42},
		Value: itemBytes,
	}
	it.Next()

	return measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           iterator.Value,
			itemsToAdd:  []any{stackitem.NewInterop(it)},
			amountToPop: 1,
		}, nil)
	})
}

func genIteratorValueRefsDelta(nopNs float64) [][]string {
	measure := func(arraySize int) []string {
		ns := iteratorValueNsPerOp(stackitem.Make(make([]any, arraySize)))
		return []string{strconv.Itoa(arraySize + 1), "0", strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepArraySize := vm.MaxStackSize / countPoints
	currArraySize := stepArraySize - 1
	rows := make([][]string, 0, countPoints+1)
	rows = append(rows, measure(0))
	for range countPoints {
		rows = append(rows, measure(currArraySize))
		currArraySize += stepArraySize
	}
	return rows
}

func genIteratorValueNumBytes(nopNs float64) [][]string {
	measure := func(numBytes int) []string {
		ns := iteratorValueNsPerOp(stackitem.Make(make([]byte, numBytes)))
		return []string{"1", strconv.Itoa(numBytes), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepNumBytes := stackitem.MaxSize / countPoints
	currNumBytes := stepNumBytes
	rows := make([][]string, 0, countPoints+1)
	rows = append(rows, measure(0))
	for range countPoints {
		rows = append(rows, measure(currNumBytes))
		currNumBytes += stepNumBytes
	}
	return rows
}

// --- System.Runtime.* ---

func genBurnGas(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.BurnGas,
			itemsToAdd:  []any{int64(1)},
			amountToPop: 0,
		}, nil)
	})
	return ns / nopNs
}

func genGasLeft(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GasLeft,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetNotificationsRefsDelta(nopNs float64) [][]string {
	ic := &interop.Context{}
	ic.SpawnVM()

	measure := func(arraySize int) []string {
		items := make([]stackitem.Item, arraySize)
		for j := range items {
			items[j] = stackitem.Null{}
		}
		ic.Notifications = []state.NotificationEvent{
			{Item: stackitem.NewArray(items)},
		}

		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           runtime.GetNotifications,
				itemsToAdd:  []any{stackitem.Null{}},
				amountToPop: 1,
			}, nil)
		})
		// Exactly one notification is created above.
		return []string{"1", strconv.Itoa(arraySize), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepArraySize := vm.MaxStackSize / countPoints
	currArraySize := stepArraySize - 1
	rows := make([][]string, 0, countPoints+1)
	// An empty notification Item array is a valid minimal value.
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currArraySize))
		currArraySize += stepArraySize
	}
	return rows
}

func genLoadScriptScriptLen(nopNs float64) [][]string {
	measure := func(scriptLen int) []string {
		script := make([]byte, scriptLen)
		for j := range script {
			script[j] = byte(opcode.RET)
		}
		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:          runtime.LoadScript,
				itemsToAdd: []any{[]any{}, int(callflag.All), script},
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes its first RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		// args_count=0, refs_delta=0.
		return []string{strconv.Itoa(scriptLen), "0", "0", strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepScriptLen := stackitem.MaxSize / countPoints
	currScriptLen := stepScriptLen
	rows := make([][]string, 0, countPoints+1)
	// isStepNeeded relies on running the loaded script's RET, so the
	// script needs at least 1 byte (an empty script has no RET to run).
	rows = append(rows, measure(1))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currScriptLen))
		currScriptLen += stepScriptLen
	}
	return rows
}

func genLoadScriptArgsCount(nopNs float64) [][]string {
	script := []byte{byte(opcode.RET)}

	measure := func(argsCount int) []string {
		args := make([]any, argsCount)
		for j := range args {
			args[j] = j
		}
		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           runtime.LoadScript,
				itemsToAdd:  []any{args, int(callflag.All), script},
				amountToPop: argsCount,
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes the RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		return []string{"1", strconv.Itoa(argsCount), strconv.Itoa(argsCount), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepArgsCount := vm.MaxStackSize / countPoints
	currArgsCount := stepArgsCount - 1
	rows := make([][]string, 0, countPoints+1)
	// 0 args is a valid minimal value.
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currArgsCount))
		currArgsCount += stepArgsCount
	}
	return rows
}

func genLoadScriptRefsDelta(nopNs float64) [][]string {
	script := []byte{byte(opcode.RET)}

	measure := func(arraySize int) []string {
		arg := stackitem.Make(make([]any, arraySize))
		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           runtime.LoadScript,
				itemsToAdd:  []any{[]any{arg}, int(callflag.All), script},
				amountToPop: 1,
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes the RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		// script_len=1 (script is a single RET), args_count=1.
		return []string{"1", "1", strconv.Itoa(arraySize), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepArraySize := vm.MaxStackSize / countPoints
	currArraySize := stepArraySize - 1
	rows := make([][]string, 0, countPoints+1)
	// An empty array argument is a valid minimal value.
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currArraySize))
		currArraySize += stepArraySize
	}
	return rows
}

func genGetAddressVersion(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetAddressVersion,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetCallingScriptHash(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetCallingScriptHash,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetEntryScriptHash(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetEntryScriptHash,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetExecutingScriptHash(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetExecutingScriptHash,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetInvocationCounter(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetInvocationCounter,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetNetwork(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetNetwork,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetRandom(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetRandom,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetScriptContainer(nopNs float64) float64 {
	ic := &interop.Context{
		Container: &transaction.Transaction{
			Signers: []transaction.Signer{{}},
		},
	}
	ic.SpawnVM()

	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, ic, benchCase{
			f:           runtime.GetScriptContainer,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func getMaxSignersCount() int {
	i := sort.Search(transaction.MaxTransactionSize, func(n int) bool {
		tx := transaction.New(nil, 0)
		tx.Signers = make([]transaction.Signer, n)
		return tx.Size() > transaction.MaxTransactionSize
	})
	return i - 1
}

func genCurrentSigners(nopNs float64) [][]string {
	ic := &interop.Context{}
	ic.SpawnVM()

	measure := func(signersCount int) []string {
		tx := transaction.New(nil, 0)
		tx.Signers = make([]transaction.Signer, signersCount)
		ic.Container = tx

		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           runtime.CurrentSigners,
				amountToPop: 1,
			}, nil)
		})
		return []string{strconv.Itoa(signersCount), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	maxSignersCount := getMaxSignersCount()
	stepSignersCount := maxSignersCount / countPoints
	currSignersCount := stepSignersCount
	rows := make([][]string, 0, countPoints+1)
	// 0 signers is a valid minimal value.
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currSignersCount))
		currSignersCount += stepSignersCount
	}
	return rows
}

func genLog(nopNs float64) float64 {
	ic := &interop.Context{Log: zap.NewNop()}
	ic.SpawnVM()
	msg := string(make([]byte, runtime.MaxNotificationSize))

	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, ic, benchCase{
			f:          runtime.Log,
			itemsToAdd: []any{msg},
		}, nil)
	})
	return ns / nopNs
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

func genNotifyEventsCount(nopNs float64) [][]string {
	measure := func(eventsCount int) []string {
		events := make([]manifest.Event, eventsCount)
		for j := range events {
			events[j] = manifest.Event{Name: fmt.Sprintf("e%d", j)}
		}
		events[eventsCount-1] = manifest.Event{Name: "target"}
		m := manifest.NewManifest("caller")
		m.ABI.Events = events

		ns := measureNsPerOp(func(b *testing.B) {
			script := []byte{byte(opcode.RET)}
			nefFile, err := nef.NewFile(script)
			if err != nil {
				b.Fatal(err)
			}
			d := dao.NewSimple(storage.NewMemoryStore(), false)
			ic := &interop.Context{DAO: d}
			ic.SpawnVM()
			ic.VM.LoadNEFMethod(nefFile, m, util.Uint160{}, hash.Hash160(script), callflag.All, false, 0, -1, nil, nil, false)

			benchInterop(b, ic, benchCase{
				f:          runtime.Notify,
				itemsToAdd: []any{[]any{}, "target"},
			}, nil)
		})
		// The "target" event that's actually notified has 0 parameters.
		return []string{strconv.Itoa(eventsCount), "0", strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	maxEventsCount := getMaxManifestEventsCount()
	stepEventsCount := maxEventsCount / countPoints
	currEventsCount := stepEventsCount
	rows := make([][]string, 0, countPoints+1)
	// The notified "target" event must itself be present in the manifest,
	// so at least 1 event is required.
	rows = append(rows, measure(1))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currEventsCount))
		currEventsCount += stepEventsCount
	}
	return rows
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

func genNotifyParamsCount(nopNs float64) [][]string {
	measure := func(paramsCount int) []string {
		params := make([]manifest.Parameter, paramsCount)
		args := make([]any, paramsCount)
		for j := range params {
			params[j] = manifest.NewParameter(fmt.Sprintf("p%d", j), smartcontract.AnyType)
			args[j] = stackitem.Null{}
		}
		m := manifest.NewManifest("caller")
		m.ABI.Events = []manifest.Event{{Name: "target", Parameters: params}}

		ns := measureNsPerOp(func(b *testing.B) {
			script := []byte{byte(opcode.RET)}
			nefFile, err := nef.NewFile(script)
			if err != nil {
				b.Fatal(err)
			}
			d := dao.NewSimple(storage.NewMemoryStore(), false)
			ic := &interop.Context{DAO: d}
			ic.SpawnVM()
			ic.VM.LoadNEFMethod(nefFile, m, util.Uint160{}, hash.Hash160(script), callflag.All, false, 0, -1, nil, nil, false)

			benchInterop(b, ic, benchCase{
				f:          runtime.Notify,
				itemsToAdd: []any{args, "target"},
			}, nil)
		})
		// Exactly one event ("target") is defined in the manifest.
		return []string{"1", strconv.Itoa(paramsCount), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	maxParamsCount := getMaxNotifyArgsCount()
	stepParamsCount := maxParamsCount / countPoints
	currParamsCount := stepParamsCount
	rows := make([][]string, 0, countPoints+1)
	// 0 event parameters is a valid minimal value.
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currParamsCount))
		currParamsCount += stepParamsCount
	}
	return rows
}

func genGetTime(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetTime,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetTrigger(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.GetTrigger,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genPlatform(nopNs float64) float64 {
	ns := measureNsPerOp(func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.Platform,
			amountToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func benchmarkNOP(b *testing.B, ic *interop.Context) {
	script := []byte{byte(opcode.NOP), byte(opcode.RET)}
	for b.Loop() {
		b.StopTimer()
		ic.VM.LoadScript(script)
		b.StartTimer()

		if err := ic.VM.Step(); err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		if err := ic.VM.Step(); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

// --- Price CSV generation ---

var (
	genBenchmarks = flag.String("genbenchmarks", "", "comma-separated list of interop names to generate price CSVs for")
	genOutDir     = flag.String("genoutdir", ".", "directory to write <interop>.csv files into")
)

func measureNsPerOp(f func(b *testing.B)) float64 {
	return float64(testing.Benchmark(f).NsPerOp())
}

func writeCSV(dir, name string, header []string, rows [][]string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, name+".csv"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return err
	}
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

type depGenerator struct {
	name string
	run  func(nopNs float64) [][]string
}

var interopGenerators = map[string][]depGenerator{
	"CreateMultisigAccount": {
		{"pubkeys_count", genCreateMultisigAccount},
	},
	"ContractCall": {
		{"name_len", genContractCallNameLen},
		{"args_count", genContractCallArgsCount},
		{"permissions_count", genContractCallPermissionsCount},
		{"methods_count", genContractCallMethodsCount},
	},
	"ContractCallNative": {
		{"refs_delta", genContractCallNativeRefsDelta},
	},
	"CheckMultisig": {
		{"keys_count", genCheckMultisig},
	},
	"IteratorValue": {
		{"refs_delta", genIteratorValueRefsDelta},
		{"num_bytes", genIteratorValueNumBytes},
	},
	"GetNotifications": {
		{"notifications_count", genGetNotifications},
		{"refs_delta", genGetNotificationsRefsDelta},
	},
	"LoadScript": {
		{"script_len", genLoadScriptScriptLen},
		{"args_count", genLoadScriptArgsCount},
		{"refs_delta", genLoadScriptRefsDelta},
	},
	"CurrentSigners": {
		{"signers_count", genCurrentSigners},
	},
	"Notify": {
		{"events_count", genNotifyEventsCount},
		{"params_count", genNotifyParamsCount},
	},
}

var staticGenerators = map[string]func(nopNs float64) float64{
	"CreateStandardAccount":  genCreateStandardAccount,
	"GetCallFlags":           genGetCallFlags,
	"CheckSig":               genCheckSig,
	"IteratorNext":           genIteratorNext,
	"BurnGas":                genBurnGas,
	"GasLeft":                genGasLeft,
	"GetAddressVersion":      genGetAddressVersion,
	"GetCallingScriptHash":   genGetCallingScriptHash,
	"GetEntryScriptHash":     genGetEntryScriptHash,
	"GetExecutingScriptHash": genGetExecutingScriptHash,
	"GetInvocationCounter":   genGetInvocationCounter,
	"GetNetwork":             genGetNetwork,
	"GetRandom":              genGetRandom,
	"GetScriptContainer":     genGetScriptContainer,
	"Log":                    genLog,
	"GetTime":                genGetTime,
	"GetTrigger":             genGetTrigger,
	"Platform":               genPlatform,
}

func combineRows(deps []depGenerator, nopNs float64) (header []string, rows [][]string) {
	for _, dep := range deps {
		header = append(header, dep.name)
	}
	header = append(header, "ns")

	for _, dep := range deps {
		rows = append(rows, dep.run(nopNs)...)
	}
	return header, rows
}

func TestGenerateInteropPriceCSV(t *testing.T) {
	names := strings.FieldsFunc(*genBenchmarks, func(r rune) bool { return r == ',' })
	if len(names) == 0 {
		for name := range interopGenerators {
			names = append(names, name)
		}
		for name := range staticGenerators {
			names = append(names, name)
		}
		sort.Strings(names)
	}

	nopIC := &interop.Context{}
	nopIC.SpawnVM()
	nopIC.VM.SetGasLimit(-1)
	nopNs := measureNsPerOp(func(b *testing.B) { benchmarkNOP(b, nopIC) })
	t.Logf("nop ns/op = %v", nopNs)

	for _, name := range names {
		if deps, ok := interopGenerators[name]; ok {
			header, rows := combineRows(deps, nopNs)
			if err := writeCSV(*genOutDir, name, header, rows); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if gen, ok := staticGenerators[name]; ok {
			row := []string{strconv.FormatFloat(gen(nopNs), 'f', 6, 64)}
			if err := writeCSV(*genOutDir, name, []string{"ns"}, [][]string{row}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		t.Fatalf("unknown interop: %s", name)
	}
}

func genGetNotifications(nopNs float64) [][]string {
	measure := func(notificationsCount int) []string {
		ic := &interop.Context{}
		ic.SpawnVM()
		notifications := make([]state.NotificationEvent, notificationsCount)
		for j := range notifications {
			notifications[j] = state.NotificationEvent{Item: stackitem.NewArray(nil)}
		}
		ic.Notifications = notifications

		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           runtime.GetNotifications,
				itemsToAdd:  []any{stackitem.Null{}},
				amountToPop: 1,
			}, nil)
		})
		// Each notification's Item is an empty array, so refs_delta is really 0.
		return []string{strconv.Itoa(notificationsCount), "0", strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepNotificationsCount := vm.MaxStackSize / countPoints
	currNotificationsCount := stepNotificationsCount
	rows := make([][]string, 0, countPoints+1)
	// 0 notifications is a valid minimal value.
	rows = append(rows, measure(0))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currNotificationsCount))
		currNotificationsCount += stepNotificationsCount
	}
	return rows
}

func genCheckMultisig(nopNs float64) [][]string {
	priv, err := keys.NewPrivateKey()
	if err != nil {
		panic(err)
	}

	d := dao.NewSimple(storage.NewMemoryStore(), false)
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), DAO: d}
	tx := transaction.New([]byte{0, 1, 2}, 1)
	ic.Container = tx
	sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
	pub := priv.PublicKey().Bytes()
	ic.SpawnVM()

	measure := func(keysCount int) []string {
		pubs := make([]any, keysCount)
		sigs := make([]any, keysCount)
		for j := range pubs {
			pubs[j] = pub
			sigs[j] = sign
		}

		ns := measureNsPerOp(func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           crypto.ECDSASecp256r1CheckMultisig,
				itemsToAdd:  []any{sigs, pubs},
				amountToPop: 1,
			}, nil)
		})
		return []string{strconv.Itoa(keysCount), strconv.FormatFloat(ns/nopNs, 'f', 6, 64)}
	}

	stepKeysCount := vm.MaxStackSize / 2 / countPoints
	currKeysCount := stepKeysCount
	rows := make([][]string, 0, countPoints+1)
	// PopSigElements rejects an empty array, so at least 1 key/signature is required.
	rows = append(rows, measure(1))
	for i := 1; i <= countPoints; i++ {
		rows = append(rows, measure(currKeysCount))
		currKeysCount += stepKeysCount
	}
	return rows
}
