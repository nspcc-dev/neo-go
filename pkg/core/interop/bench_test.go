package interop_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
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
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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
	var (
		benchCases []benchCase
		step       = vm.MaxStackSize / countPoints
		curr       = step
		pubBytes   = priv.PublicKey().Bytes()
	)

	for range countPoints {
		pubs := make([]any, 0, curr)
		for range curr {
			pubs = append(pubs, pubBytes)
		}
		benchCases = append(benchCases, benchCase{
			name:        fmt.Sprintf("%d", curr),
			f:           contract.CreateMultisigAccount,
			itemsToAdd:  []any{pubs, 1},
			amountToPop: 1,
		})
		curr += step
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchInterop(b, nil, bc, nil)
		})
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
		itemsToAdd:  []any{pub, sign},
		amountToPop: 1,
	}, nil)
}

// --- System.Iterator.* ---

func BenchmarkIteratorNext(b *testing.B) {
	benchInterop(b, nil, benchCase{
		f:           iterator.Next,
		itemsToAdd:  []any{istorage.NewIterator(make(chan storage.KeyValue), nil, 0)},
		amountToPop: 1,
	}, nil)
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
	ic := &interop.Context{}
	ic.Container = &transaction.Transaction{
		Signers: []transaction.Signer{{Account: random.Uint160()}},
		Scripts: []transaction.Witness{{}},
	}
	ic.SpawnVM()

	benchInterop(b, nil, benchCase{
		f:           runtime.GetScriptContainer,
		amountToPop: 1,
	}, nil)
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
