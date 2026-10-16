package interop_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
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

const (
	// countPoints is the number of equal steps a parameter range is split into,
	// so every dependency is measured at countPoints+1 points including both
	// ends of the range.
	countPoints = 8
	// maxSubitems mirrors transaction's own private maxSubitems limit on
	// Signer.AllowedContracts, Signer.AllowedGroups, Signer.Rules and the
	// width of a single AND/OR condition.
	maxSubitems = 16
)

// --- System.Contract.* ---

func genContractCallNameLen(t *testing.T, nopNs float64) []sample {
	measure := func(nameLen int) sample {
		name := make([]byte, nameLen)
		cs := &state.Contract{
			ContractBase: state.ContractBase{
				Hash: random.Uint160(),
				Manifest: manifest.Manifest{
					ABI: manifest.ABI{
						Methods: []manifest.Method{
							{
								Name:       string(name),
								ReturnType: smartcontract.VoidType,
							},
						},
					},
				},
			},
		}
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           contract.Call,
				itemsToPush: []any{[]any{}, int32(0), name, cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		return sample{deps: []int{nameLen, 0, 0, 1, 0}, value: ns / nopNs}
	}

	step := manifest.MaxManifestSize / countPoints
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genContractCallArgsCount(t *testing.T, nopNs float64) []sample {
	measure := func(argsCount int) sample {
		args := make([]any, argsCount)
		cs := newContractWithParamCount(argsCount)
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           contract.Call,
				itemsToPush: []any{args, int32(0), "", cs.Hash},
				numToPop:    argsCount,
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		return sample{deps: []int{0, argsCount, 0, 1, 0}, value: ns / nopNs}
	}

	maxArgsCount := getMaxContractParamCount()
	step := maxArgsCount / countPoints
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func getMaxContractParamCount() int {
	i := sort.Search(vm.MaxStackSize, func(paramCount int) bool {
		cs := newContractWithParamCount(paramCount)
		d := dao.NewSimple(storage.NewMemoryStore(), false)
		return d.PutStorageConvertible(1, []byte{1}, cs) != nil
	})
	return i - 1
}

func genContractCallPermissionsCount(t *testing.T, nopNs float64) []sample {
	measure := func(permissionsCount int) sample {
		cs := newContractWithParamCount(0)

		callerManifest := manifest.NewManifest("caller")
		callerManifest.Permissions = make([]manifest.Permission, permissionsCount)
		for j := range permissionsCount - 1 {
			callerManifest.Permissions[j] = *manifest.NewPermission(manifest.PermissionHash, random.Uint160())
		}
		callerManifest.Permissions[permissionsCount-1] = *manifest.NewPermission(manifest.PermissionHash, cs.Hash)

		ns := measureNsPerOp(t, func(b *testing.B) {
			blkChain, _ := chain.NewSingle(b)
			ic, err := blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
			require.NoError(b, err)
			callerScript := []byte{byte(opcode.RET)}
			callerNEF, err := nef.NewFile(callerScript)
			require.NoError(b, err)
			ic.VM.LoadNEFMethod(callerNEF, callerManifest, util.Uint160{}, hash.Hash160(callerScript),
				callflag.All, false, 0, -1, nil, nil, false)

			benchInterop(b, ic, benchCase{
				f:           contract.Call,
				itemsToPush: []any{[]any{}, int32(0), "", cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		return sample{deps: []int{0, 0, permissionsCount, 1, 0}, value: ns / nopNs}
	}

	maxPermissionsCount := getMaxManifestPermissionsCount()
	step := (maxPermissionsCount - 1) / countPoints
	curr := 1
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
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

func genContractCallMethodsCount(t *testing.T, nopNs float64) []sample {
	measure := func(methodsCount int) sample {
		cs := newContractWithMethodCount(methodsCount)
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           contract.Call,
				itemsToPush: []any{[]any{}, int32(0), "", cs.Hash},
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		return sample{deps: []int{0, 0, 0, methodsCount, 0}, value: ns / nopNs}
	}

	maxMethodsCount := getMaxMethodsCount()
	step := (maxMethodsCount - 1) / countPoints
	curr := 1
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func getMaxMethodsCount() int {
	i := sort.Search(vm.MaxStackSize, func(methodCount int) bool {
		cs := newContractWithMethodCount(methodCount)
		d := dao.NewSimple(storage.NewMemoryStore(), false)
		return d.PutStorageConvertible(1, []byte{1}, cs) != nil
	})
	return i - 1
}

func newContractWithMethodCount(methodCount int) *state.Contract {
	methods := make([]manifest.Method, methodCount)
	for i := range methods {
		methods[i] = manifest.Method{Name: strconv.Itoa(i), ReturnType: smartcontract.VoidType}
	}
	if methodCount > 0 {
		methods[methodCount-1] = manifest.Method{Name: "", ReturnType: smartcontract.VoidType}
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

func genContractCallRefsDelta(t *testing.T, nopNs float64) []sample {
	measure := func(arraySize int) sample {
		cs := newContractWithParamCount(1)
		inner := make([]any, arraySize)
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           contract.Call,
				itemsToPush: []any{[]any{inner}, int32(0), "", cs.Hash},
				numToPop:    1,
				// Call pushes a new context onto Istack.
				// One Step executes single RET and unloads it.
				isStepNeeded: true,
			}, cs)
		})
		return sample{deps: []int{0, 1, 0, 1, arraySize}, value: ns / nopNs}
	}

	step := vm.MaxStackSize / countPoints
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
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
							ReturnType: smartcontract.VoidType,
						},
					},
				},
			},
		},
	}
}

func genContractCallNativeRefsDelta(t *testing.T, nopNs float64) []sample {
	measure := func(arraySize int) sample {
		items := make([]stackitem.Item, 0, arraySize)
		for range arraySize {
			items = append(items, stackitem.Null{})
		}
		arr := stackitem.NewArray(items)
		ns := measureNsPerOp(t, func(b *testing.B) {
			blkChain, _ := chain.NewSingle(b)
			ic, err := blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
			require.NoError(b, err)
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
					callflag.All, false, method.SyscallOffset, -1, nil, nil, false)
				ic.VM.Estack().PushItem(arr)
				ic.VM.Estack().PushVal(0)

				b.StartTimer()
				err := ic.VM.Step()
				b.StopTimer()
				require.NoError(b, err)

				ic.VM.Estack().Pop()
				b.StartTimer()
			}
		})
		return sample{deps: []int{arraySize + 1}, value: ns / nopNs}
	}

	step := (vm.MaxStackSize - 2) / countPoints
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genCreateMultisigAccount(t *testing.T, nopNs float64) []sample {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubBytes := priv.PublicKey().Bytes()

	measure := func(n int) sample {
		pubs := make([]any, n)
		for j := range pubs {
			pubs[j] = pubBytes
		}
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           contract.CreateMultisigAccount,
				itemsToPush: []any{pubs, 1},
				numToPop:    1,
			}, nil)
		})
		return sample{deps: []int{n}, value: ns / nopNs}
	}

	step := (vm.MaxStackSize - 1) / countPoints
	curr := 1
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genCreateStandardAccount(t *testing.T, nopNs float64) float64 {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubBytes := priv.PublicKey().Bytes()

	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           contract.CreateStandardAccount,
			itemsToPush: []any{pubBytes},
			numToPop:    1,
		}, nil)
	})
	return ns / nopNs
}

func genGetCallFlags(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        contract.GetCallFlags,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

// --- System.Crypto.* ---

func genCheckMultisig(t *testing.T, nopNs float64) []sample {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	d := dao.NewSimple(storage.NewMemoryStore(), false)
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), DAO: d}
	tx := transaction.New([]byte{0, 1, 2}, 1)
	ic.Container = tx
	sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
	pub := priv.PublicKey().Bytes()
	ic.SpawnVM()

	measure := func(keysCount int) sample {
		pubs := make([]any, keysCount)
		sigs := make([]any, keysCount)
		for j := range pubs {
			pubs[j] = pub
			sigs[j] = sign
		}

		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           crypto.ECDSASecp256r1CheckMultisig,
				itemsToPush: []any{sigs, pubs},
				numToPop:    1,
			}, nil)
		})
		return sample{deps: []int{keysCount}, value: ns / nopNs}
	}

	step := (vm.MaxStackSize/2 - 1) / countPoints
	curr := 1
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genCheckSig(t *testing.T, nopNs float64) float64 {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	d := dao.NewSimple(storage.NewMemoryStore(), false)
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), DAO: d}
	tx := transaction.New([]byte{0, 1, 2}, 1)
	ic.Container = tx
	sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
	pub := priv.PublicKey().Bytes()

	ic.SpawnVM()
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, ic, benchCase{
			f:           crypto.ECDSASecp256r1CheckSig,
			itemsToPush: []any{sign, pub},
			numToPop:    1,
		}, nil)
	})
	return ns / nopNs
}

// --- System.Iterator.* ---

func genIteratorNext(t *testing.T, nopNs float64) float64 {
	ch := make(chan storage.KeyValue)
	close(ch)
	it := stackitem.NewInterop(istorage.NewIterator(ch, nil, 0))
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           iterator.Next,
			itemsToPush: []any{it},
			numToPop:    1,
		}, nil)
	})
	return ns / nopNs
}

func genIteratorValueRefsDelta(t *testing.T, nopNs float64) []sample {
	measure := func(arraySize int) sample {
		ns := iteratorValueNsPerOp(t, stackitem.Make(make([]any, arraySize)))
		return sample{deps: []int{arraySize + 1, 0}, value: ns / nopNs}
	}

	step := (vm.MaxStackSize - 1) / countPoints
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genIteratorValueNumBytes(t *testing.T, nopNs float64) []sample {
	measure := func(numBytes int) sample {
		ns := iteratorValueNsPerOp(t, stackitem.Make(make([]byte, numBytes)))
		return sample{deps: []int{1, numBytes}, value: ns / nopNs}
	}

	step := stackitem.MaxSize / countPoints
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func iteratorValueNsPerOp(t *testing.T, item stackitem.Item) float64 {
	itemBytes, err := stackitem.Serialize(item)
	require.NoError(t, err)
	ch := make(chan storage.KeyValue, 1)
	it := istorage.NewIterator(ch, nil, istorage.FindDeserialize|istorage.FindValuesOnly)

	ch <- storage.KeyValue{
		Key:   []byte{42},
		Value: itemBytes,
	}
	it.Next()

	return measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           iterator.Value,
			itemsToPush: []any{stackitem.NewInterop(it)},
			numToPop:    1,
		}, nil)
	})
}

// --- System.Runtime.* ---

func genBurnGas(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           runtime.BurnGas,
			itemsToPush: []any{int64(1)},
			numToPop:    0,
		}, nil)
	})
	return ns / nopNs
}

func genCheckWitnessRulesCount(t *testing.T, nopNs float64) []sample {
	measure := func(rulesCount int) sample {
		ns := measureNsPerOp(t, func(b *testing.B) {
			checkWitnessBenchCase(b, 0, rulesCount, nil)
		})
		return sample{deps: []int{rulesCount, 0}, value: ns / nopNs}
	}

	// One point less to cover the whole range: 1, 3, ..., 15.
	step := (maxSubitems - 1) / (countPoints - 1)
	curr := 1
	rows := make([]sample, 0, countPoints+1)
	rows = append(rows, sample{deps: []int{0, 0}, value: genCheckWitnessCalledByEntry(t, nopNs)})
	for range countPoints {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genCheckWitnessGroupsCount(t *testing.T, nopNs float64) []sample {
	groupKey, err := keys.NewPrivateKey()
	require.NoError(t, err)
	groupPub := groupKey.PublicKey()
	maxGroupsCount := getMaxManifestGroupsCount(groupPub)

	measure := func(groupsCount int) sample {
		ns := measureNsPerOp(t, func(b *testing.B) {
			checkWitnessBenchCase(b, groupsCount, maxSubitems, groupPub)
		})
		return sample{deps: []int{maxSubitems, groupsCount}, value: ns / nopNs}
	}

	step := maxGroupsCount / countPoints
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func getMaxManifestGroupsCount(pub *keys.PublicKey) int {
	i := sort.Search(manifest.MaxManifestSize, func(n int) bool {
		m := manifest.NewManifest("target")
		m.Groups = make([]manifest.Group, n)
		for j := range n {
			m.Groups[j] = manifest.Group{PublicKey: pub, Signature: random.Bytes(64)}
		}
		data, _ := json.Marshal(m)
		return len(data) > manifest.MaxManifestSize
	})
	return i - 1
}

func checkWitnessBenchCase(b *testing.B, groupsCount, rulesCount int, groupPub *keys.PublicKey) {
	blkChain, _ := chain.NewSingle(b)
	ic, err := blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
	require.NoError(b, err)

	targetHash := random.Uint160()
	checkedHash := random.Uint160()

	groups := make([]manifest.Group, groupsCount)
	for i := range groups {
		groups[i] = manifest.Group{PublicKey: groupPub}
	}
	cs := &state.Contract{
		ContractBase: state.ContractBase{
			Hash:     targetHash,
			Manifest: manifest.Manifest{Groups: groups},
		},
	}

	allowedGroupKey, err := keys.NewPrivateKey()
	require.NoError(b, err)
	allowedGroups := make([]*keys.PublicKey, maxSubitems)
	for i := range allowedGroups {
		allowedGroups[i] = allowedGroupKey.PublicKey()
	}
	allowedContracts := make([]util.Uint160, maxSubitems)
	for i := range allowedContracts {
		allowedContracts[i] = random.Uint160()
	}
	rules := make([]transaction.WitnessRule, rulesCount)
	for i := range rules {
		top := make(transaction.ConditionOr, maxSubitems)
		for j := range top {
			mid := make(transaction.ConditionOr, maxSubitems)
			for k := range mid {
				h := random.Uint160()
				csh := transaction.ConditionScriptHash(h)
				mid[k] = &csh
			}
			top[j] = &mid
		}
		rules[i] = transaction.WitnessRule{Action: transaction.WitnessDeny, Condition: &top}
	}
	signers := make([]transaction.Signer, transaction.MaxAttributes)
	for i := range transaction.MaxAttributes - 1 {
		signers[i] = transaction.Signer{Account: random.Uint160()}
	}
	signers[transaction.MaxAttributes-1] = transaction.Signer{
		Account: checkedHash,
		Scopes: transaction.CalledByEntry | transaction.CustomContracts |
			transaction.CustomGroups | transaction.Rules,
		AllowedContracts: allowedContracts,
		AllowedGroups:    allowedGroups,
		Rules:            rules,
	}
	ic.Tx.Signers = signers

	ic.VM.LoadScriptWithHash([]byte{byte(opcode.RET)}, random.Uint160(), callflag.All)
	ic.VM.LoadScriptWithHash([]byte{byte(opcode.RET)}, random.Uint160(), callflag.All)
	ic.VM.LoadScriptWithHash([]byte{byte(opcode.RET)}, targetHash, callflag.All)

	benchInterop(b, ic, benchCase{
		f:           runtime.CheckWitness,
		itemsToPush: []any{checkedHash.BytesBE()},
		numToPop:    1,
	}, cs)
}

func genCheckWitnessCalledByEntry(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		blkChain, _ := chain.NewSingle(b)
		ic, err := blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
		require.NoError(b, err)

		checkedHash := random.Uint160()
		signers := make([]transaction.Signer, transaction.MaxAttributes)
		for i := range transaction.MaxAttributes - 1 {
			signers[i] = transaction.Signer{Account: random.Uint160()}
		}
		signers[transaction.MaxAttributes-1] = transaction.Signer{
			Account: checkedHash,
			Scopes:  transaction.CalledByEntry,
		}
		ic.Tx.Signers = signers

		ic.VM.LoadScriptWithHash([]byte{byte(opcode.RET)}, random.Uint160(), callflag.All)

		benchInterop(b, ic, benchCase{
			f:           runtime.CheckWitness,
			itemsToPush: []any{checkedHash.BytesBE()},
			numToPop:    1,
		}, nil)
	})
	return ns / nopNs
}

func genCurrentSignersManySigners(t *testing.T, nopNs float64) []sample {
	groupKey, err := keys.NewPrivateKey()
	require.NoError(t, err)
	groupPub := groupKey.PublicKey()

	allowedGroups := make([]*keys.PublicKey, maxSubitems)
	for i := range allowedGroups {
		allowedGroups[i] = groupPub
	}
	allowedContracts := make([]util.Uint160, maxSubitems)
	for i := range allowedContracts {
		allowedContracts[i] = random.Uint160()
	}
	rules := make([]transaction.WitnessRule, maxSubitems)
	for i := range rules {
		csh := transaction.ConditionScriptHash(random.Uint160())
		rules[i] = transaction.WitnessRule{Action: transaction.WitnessDeny, Condition: &csh}
	}
	// Signer (6), contracts and groups (1 each), rules (array, action, ScriptHash (array, type, hash)).
	signerRefs := 6 + 2*maxSubitems + 5*maxSubitems

	measure := func(signersCount int) sample {
		signers := make([]transaction.Signer, signersCount)
		for i := range signers {
			signers[i] = transaction.Signer{
				Account:          random.Uint160(),
				Scopes:           transaction.CustomContracts | transaction.CustomGroups | transaction.Rules,
				AllowedContracts: allowedContracts,
				AllowedGroups:    allowedGroups,
				Rules:            rules,
			}
		}
		return sample{deps: []int{1 + signersCount*signerRefs}, value: measureCurrentSigners(t, signers, nopNs)}
	}

	// One point less to cover the whole range: 1, 3, ..., 15.
	step := (transaction.MaxAttributes - 1) / (countPoints - 1)
	curr := 1
	rows := make([]sample, 0, countPoints)
	for range countPoints {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genCurrentSignersOneSigner(t *testing.T, nopNs float64) []sample {
	newSignerWithRefs := func(refsDelta int) (transaction.Signer, int) {
		leftRefs := refsDelta - 6 // one signer has 6 references by default
		rules := make([]transaction.WitnessRule, 0, maxSubitems)
		for leftRefs > 0 && len(rules) < maxSubitems {
			leftRefs -= 2 + 3 // rule (array, action) and top Or (array, type, operands array)
			top := make(transaction.ConditionOr, 0, maxSubitems)
			for len(top) == 0 || leftRefs > 0 && len(top) < maxSubitems {
				leftRefs -= 3 // mid Or (array, type, operands array)
				mid := make(transaction.ConditionOr, 0, maxSubitems)
				for len(mid) == 0 || leftRefs > 0 && len(mid) < maxSubitems {
					leftRefs -= 3 // ScriptHash (array, type, hash)
					csh := transaction.ConditionScriptHash(random.Uint160())
					mid = append(mid, &csh)
				}
				top = append(top, &mid)
			}
			rules = append(rules, transaction.WitnessRule{Action: transaction.WitnessDeny, Condition: &top})
		}
		signer := transaction.Signer{
			Account: random.Uint160(),
			Scopes:  transaction.Rules,
			Rules:   rules,
		}
		return signer, refsDelta - leftRefs
	}

	measure := func(refsDelta int) sample {
		signer, actualRefsDelta := newSignerWithRefs(refsDelta)
		signers := []transaction.Signer{signer}
		return sample{deps: []int{1 + actualRefsDelta}, value: measureCurrentSigners(t, signers, nopNs)}
	}

	step := vm.MaxStackSize / countPoints
	rows := make([]sample, 0, countPoints+1)
	for refsDelta := 0; refsDelta <= vm.MaxStackSize; refsDelta += step {
		rows = append(rows, measure(refsDelta))
	}
	return rows
}

func measureCurrentSigners(t *testing.T, signers []transaction.Signer, nopNs float64) float64 {
	ic := &interop.Context{}
	ic.SpawnVM()

	tx := transaction.New(nil, 0)
	tx.Signers = signers
	ic.Container = tx

	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, ic, benchCase{
			f:        runtime.CurrentSigners,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGasLeft(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GasLeft,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetAddressVersion(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GetAddressVersion,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetCallingScriptHash(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GetCallingScriptHash,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetEntryScriptHash(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GetEntryScriptHash,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetExecutingScriptHash(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GetExecutingScriptHash,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetInvocationCounter(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GetInvocationCounter,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetNetwork(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GetNetwork,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetNotificationsCount(t *testing.T, nopNs float64) []sample {
	measure := func(notificationsCount int) sample {
		ic := &interop.Context{}
		ic.SpawnVM()
		notifications := make([]state.NotificationEvent, notificationsCount)
		for j := range notifications {
			notifications[j] = state.NotificationEvent{Item: stackitem.NewArray(nil)}
		}
		ic.Notifications = notifications

		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           runtime.GetNotifications,
				itemsToPush: []any{stackitem.Null{}},
				numToPop:    1,
			}, nil)
		})
		refsDelta := 1 + 4*notificationsCount // result array + array, hash, name and empty Item array per notification
		return sample{deps: []int{notificationsCount, refsDelta}, value: ns / nopNs}
	}

	step := vm.MaxStackSize / countPoints
	// 0 notifications is a valid minimal value.
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genGetNotificationsRefsDelta(t *testing.T, nopNs float64) []sample {
	ic := &interop.Context{}
	ic.SpawnVM()

	measure := func(arraySize int) sample {
		items := make([]stackitem.Item, arraySize)
		for j := range items {
			items[j] = stackitem.Null{}
		}
		ic.Notifications = []state.NotificationEvent{
			{Item: stackitem.NewArray(items)},
		}

		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, ic, benchCase{
				f:           runtime.GetNotifications,
				itemsToPush: []any{stackitem.Null{}},
				numToPop:    1,
			}, nil)
		})
		refsDelta := 1 + 1 + 3 + arraySize // result array, notification array, hash, name, Item array and its elements.
		return sample{deps: []int{1, refsDelta}, value: ns / nopNs}
	}

	step := (vm.MaxStackSize - 1) / countPoints
	// An empty notification Item array is a valid minimal value.
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genGetRandom(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GetRandom,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetScriptContainer(t *testing.T, nopNs float64) float64 {
	ic := &interop.Context{
		Container: &transaction.Transaction{
			Signers: []transaction.Signer{{}},
		},
	}
	ic.SpawnVM()

	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, ic, benchCase{
			f:        runtime.GetScriptContainer,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetTime(t *testing.T, nopNs float64) float64 {
	ic := &interop.Context{Block: &block.Block{Header: block.Header{Timestamp: 1}}}
	ic.SpawnVM()

	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, ic, benchCase{
			f:        runtime.GetTime,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genGetTrigger(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.GetTrigger,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genLoadScriptScriptLen(t *testing.T, nopNs float64) []sample {
	measure := func(scriptLen int) sample {
		script := make([]byte, scriptLen)
		for j := range script {
			script[j] = byte(opcode.RET)
		}
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           runtime.LoadScript,
				itemsToPush: []any{[]any{}, int(callflag.All), script},
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes its first RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		// args_count=0, refs_delta=0.
		return sample{deps: []int{scriptLen, 0, 0}, value: ns / nopNs}
	}

	step := (stackitem.MaxSize - 1) / countPoints
	// isStepNeeded relies on running the loaded script's RET, so the
	// script needs at least 1 byte (an empty script has no RET to run).
	curr := 1
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genLoadScriptArgsCount(t *testing.T, nopNs float64) []sample {
	script := []byte{byte(opcode.RET)}

	measure := func(argsCount int) sample {
		args := make([]any, argsCount)
		for j := range args {
			args[j] = j
		}
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           runtime.LoadScript,
				itemsToPush: []any{args, int(callflag.All), script},
				numToPop:    argsCount,
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes the RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		return sample{deps: []int{1, argsCount, argsCount}, value: ns / nopNs}
	}

	step := (vm.MaxStackSize - 1) / countPoints
	// 0 args is a valid minimal value.
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genLoadScriptRefsDelta(t *testing.T, nopNs float64) []sample {
	script := []byte{byte(opcode.RET)}

	measure := func(arraySize int) sample {
		arg := stackitem.Make(make([]any, arraySize))
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchInterop(b, nil, benchCase{
				f:           runtime.LoadScript,
				itemsToPush: []any{[]any{arg}, int(callflag.All), script},
				numToPop:    1,
				// LoadScript loads a dynamic context onto Istack.
				// One Step executes the RET and unloads it.
				isStepNeeded: true,
			}, nil)
		})
		// script_len=1 (script is a single RET), args_count=1.
		return sample{deps: []int{1, 1, arraySize}, value: ns / nopNs}
	}

	step := (vm.MaxStackSize - 1) / countPoints
	// An empty array argument is a valid minimal value.
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func genLog(t *testing.T, nopNs float64) float64 {
	ic := &interop.Context{Log: zap.NewNop()}
	ic.SpawnVM()
	msg := string(make([]byte, runtime.MaxNotificationSize))

	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, ic, benchCase{
			f:           runtime.Log,
			itemsToPush: []any{msg},
		}, nil)
	})
	return ns / nopNs
}

func genNotifyEventsCount(t *testing.T, nopNs float64) []sample {
	measure := func(eventsCount int) sample {
		events := make([]manifest.Event, eventsCount)
		for j := range events {
			events[j] = manifest.Event{Name: fmt.Sprintf("e%d", j)}
		}
		events[eventsCount-1] = manifest.Event{Name: "target"}
		m := manifest.NewManifest("caller")
		m.ABI.Events = events

		ns := measureNsPerOp(t, func(b *testing.B) {
			script := []byte{byte(opcode.RET)}
			nefFile, err := nef.NewFile(script)
			require.NoError(b, err)
			d := dao.NewSimple(storage.NewMemoryStore(), false)
			ic := &interop.Context{DAO: d}
			ic.SpawnVM()
			ic.VM.LoadNEFMethod(nefFile, m, util.Uint160{}, hash.Hash160(script), callflag.All, false, 0, -1, nil, nil, false)

			benchInterop(b, ic, benchCase{
				f:           runtime.Notify,
				itemsToPush: []any{[]any{}, "target"},
			}, nil)
		})
		return sample{deps: []int{eventsCount, 0, 0}, value: ns / nopNs}
	}

	maxEventsCount := getMaxManifestEventsCount()
	step := (maxEventsCount - 1) / countPoints
	curr := 1
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
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

func genNotifyParamsCount(t *testing.T, nopNs float64) []sample {
	measure := func(paramsCount int) sample {
		params := make([]manifest.Parameter, paramsCount)
		args := make([]any, paramsCount)
		for j := range params {
			params[j] = manifest.NewParameter(fmt.Sprintf("p%d", j), smartcontract.AnyType)
			args[j] = stackitem.Null{}
		}
		m := manifest.NewManifest("caller")
		m.ABI.Events = []manifest.Event{{Name: "target", Parameters: params}}

		ns := measureNsPerOp(t, func(b *testing.B) {
			script := []byte{byte(opcode.RET)}
			nefFile, err := nef.NewFile(script)
			require.NoError(b, err)
			d := dao.NewSimple(storage.NewMemoryStore(), false)
			ic := &interop.Context{DAO: d}
			ic.SpawnVM()
			ic.VM.LoadNEFMethod(nefFile, m, util.Uint160{}, hash.Hash160(script), callflag.All, false, 0, -1, nil, nil, false)

			benchInterop(b, ic, benchCase{
				f:           runtime.Notify,
				itemsToPush: []any{args, "target"},
			}, nil)
		})
		return sample{deps: []int{1, paramsCount, 0}, value: ns / nopNs}
	}

	maxParamsCount := getMaxNotifyArgsCount()
	step := maxParamsCount / countPoints
	curr := 0
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
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

func genNotifyItemsCount(t *testing.T, nopNs float64) []sample {
	params := []manifest.Parameter{manifest.NewParameter("p0", smartcontract.AnyType)}
	m := manifest.NewManifest("caller")
	m.ABI.Events = []manifest.Event{{Name: "target", Parameters: params}}

	measure := func(depth int) sample {
		item := nestedArrayItem(depth)

		ns := measureNsPerOp(t, func(b *testing.B) {
			script := []byte{byte(opcode.RET)}
			nefFile, err := nef.NewFile(script)
			require.NoError(b, err)
			d := dao.NewSimple(storage.NewMemoryStore(), false)
			ic := &interop.Context{DAO: d}
			ic.SpawnVM()
			ic.VM.LoadNEFMethod(nefFile, m, util.Uint160{}, hash.Hash160(script), callflag.All, false, 0, -1, nil, nil, false)

			benchInterop(b, ic, benchCase{
				f:           runtime.Notify,
				itemsToPush: []any{item, "target"},
			}, nil)
		})
		return sample{deps: []int{1, 1, depth}, value: ns / nopNs}
	}

	maxDepth := getMaxNotifyNestingDepth()
	step := (maxDepth - 2) / countPoints
	curr := 2
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

func getMaxNotifyNestingDepth() int {
	d := dao.NewSimple(storage.NewMemoryStore(), false)
	i := sort.Search(vm.MaxStackSize, func(n int) bool {
		bytes, err := d.GetItemCtx().Serialize(nestedArrayItem(n), false)
		if err != nil {
			return true
		}
		return len(bytes) > runtime.MaxNotificationSize
	})
	return i - 1
}

func nestedArrayItem(depth int) stackitem.Item {
	var item stackitem.Item = stackitem.Null{}
	for range depth - 1 {
		item = stackitem.NewArray([]stackitem.Item{item})
	}
	return item
}

func genPlatform(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:        runtime.Platform,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

// --- System.Storage.* ---

// dataPagesOffset is the offset of BoltDB data pages in the benchmark DBs. The
// first pages are meta and root ones, so they're not dropped from the page cache.
var dataPagesOffset = int64(3 * os.Getpagesize())

func genStorageGetFromDisk(t *testing.T, nopNs float64) float64 {
	const id = 1
	key := []byte{0x01}

	dbCfg := dbconfig.BoltDBOptions{FilePath: filepath.Join(t.TempDir(), "chain.bolt")}

	st, err := storage.NewBoltDBStore(dbCfg)
	require.NoError(t, err)
	d := dao.NewSimple(st, false)
	d.PutStorageItem(id, []byte{0x02}, make([]byte, 16*1024))
	d.PutStorageItem(id, key, []byte{0x02})
	_, err = d.Persist()
	require.NoError(t, err)
	defer func() { require.NoError(t, st.Close()) }()

	ic := &interop.Context{}
	ic.SpawnVM()
	stc := stackitem.NewInterop(&istorage.Context{ID: id})
	ns := measureNsPerOp(t, func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			require.NoError(b, dropPageCache(dbCfg.FilePath, dataPagesOffset))
			ic.DAO = dao.NewSimple(st, false).GetPrivate()
			ic.VM.Estack().PushVal(key)
			ic.VM.Estack().PushItem(stc)
			b.StartTimer()

			err := istorage.Get(ic)

			b.StopTimer()
			require.NoError(b, err)
			require.NotEqual(b, stackitem.Null{}, ic.VM.Estack().Pop().Item(), "stored record is not found")
			b.StartTimer()
		}
	})
	return ns / nopNs
}

func genStorageFindFromDisk(t *testing.T, nopNs float64) float64 {
	const id = 1
	prefix := []byte{0x01}

	dbCfg := dbconfig.BoltDBOptions{FilePath: filepath.Join(t.TempDir(), "chain.bolt")}

	st, err := storage.NewBoltDBStore(dbCfg)
	require.NoError(t, err)
	d := dao.NewSimple(st, false)
	d.PutStorageItem(id, []byte{0x02}, make([]byte, 16*1024))
	d.PutStorageItem(id, prefix, []byte{0x02})
	_, err = d.Persist()
	require.NoError(t, err)
	defer func() { require.NoError(t, st.Close()) }()

	ic := &interop.Context{}
	ic.SpawnVM()
	stc := stackitem.NewInterop(&istorage.Context{ID: id})
	ns := measureNsPerOp(t, func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			require.NoError(b, dropPageCache(dbCfg.FilePath, dataPagesOffset))
			ic.DAO = dao.NewSimple(st, false).GetPrivate()
			ic.VM.Estack().PushVal(istorage.FindDefault)
			ic.VM.Estack().PushVal(prefix)
			ic.VM.Estack().PushItem(stc)
			b.StartTimer()

			err := istorage.Find(ic)
			if err == nil {
				err = iterator.Next(ic)
			}

			b.StopTimer()
			require.NoError(b, err)
			require.True(b, ic.VM.Estack().Pop().Bool(), "stored record is not found")
			ic.Finalize()
			b.StartTimer()
		}
	})
	return ns / nopNs
}

func genStorageGetContext(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		blkChain, _ := chain.NewSingle(b)
		ic, err := blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
		require.NoError(b, err)
		ic.VM.LoadScriptWithHash([]byte{byte(opcode.RET)}, nativehashes.ContractManagement, callflag.All)

		benchInterop(b, ic, benchCase{
			f:        istorage.GetContext,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genStorageGetReadOnlyContext(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		blkChain, _ := chain.NewSingle(b)
		ic, err := blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
		require.NoError(b, err)
		ic.VM.LoadScriptWithHash([]byte{byte(opcode.RET)}, nativehashes.ContractManagement, callflag.All)

		benchInterop(b, ic, benchCase{
			f:        istorage.GetReadOnlyContext,
			numToPop: 1,
		}, nil)
	})
	return ns / nopNs
}

func genStorageAsReadOnly(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		benchInterop(b, nil, benchCase{
			f:           istorage.ContextAsReadOnly,
			itemsToPush: []any{stackitem.NewInterop(&istorage.Context{ID: 1})},
			numToPop:    1,
		}, nil)
	})
	return ns / nopNs
}
