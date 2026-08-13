package interop_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

const (
	// generatePrices enables price CSV generation. To regenerate files in
	// genOutDir, set it to true, run TestGenerateInteropPriceCSV and
	// set it back to false.
	generatePrices = false
	// genOutDir is a directory price CSV files are written to.
	genOutDir = "testdata/prices"
	// nopCount is the number of NOPs in the script NOP time is measured with.
	nopCount = 10000
)

func TestGenerateInteropPriceCSV(t *testing.T) {
	if generatePrices {
		nopIC := &interop.Context{}
		nopIC.SpawnVM()
		nopIC.VM.SetGasLimit(-1)
		nopNs := measureNsPerOp(t, func(b *testing.B) { benchmarkNOP(b, nopIC) }) / nopCount

		names := make([]string, 0, len(interopGenerators)+len(staticGenerators))
		for name := range interopGenerators {
			names = append(names, name)
		}
		for name := range staticGenerators {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			if deps, ok := interopGenerators[name]; ok {
				header, rows := combineRows(t, deps, nopNs)
				require.NoError(t, writeCSV(genOutDir, name, header, rows))
				continue
			}
			require.NoError(t, writeCSV(genOutDir, name, []string{"NOPs"}, []sample{{value: staticGenerators[name](t, nopNs)}}))
		}
	}
	require.False(t, generatePrices)
}

func benchmarkNOP(b *testing.B, ic *interop.Context) {
	script := make([]byte, nopCount+1)
	for i := range nopCount {
		script[i] = byte(opcode.NOP)
	}
	script[nopCount] = byte(opcode.RET)
	for b.Loop() {
		b.StopTimer()
		ic.VM.LoadScript(script)
		b.StartTimer()
		require.NoError(b, ic.VM.Run())
	}
}

var interopGenerators = map[string][]depGenerator{
	interopnames.SystemContractCreateMultisigAccount: {
		{"pubkeys_count", genCreateMultisigAccount},
	},
	interopnames.SystemRuntimeCheckWitness: {
		{"rules_count", genCheckWitnessRulesCount},
		{"groups_count", genCheckWitnessGroupsCount},
	},
	interopnames.SystemContractCall: {
		{"name_len", genContractCallNameLen},
		{"args_count", genContractCallArgsCount},
		{"permissions_count", genContractCallPermissionsCount},
		{"methods_count", genContractCallMethodsCount},
		{"refs_delta", genContractCallRefsDelta},
	},
	interopnames.SystemContractCallNative: {
		{"refs_delta", genContractCallNativeRefsDelta},
	},
	interopnames.SystemCryptoCheckMultisig: {
		{"keys_count", genCheckMultisig},
	},
	interopnames.SystemRuntimeCurrentSigners: {
		{"refs_delta", func(t *testing.T, nopNs float64) []sample {
			return append(genCurrentSignersManySigners(t, nopNs), genCurrentSignersOneSigner(t, nopNs)...)
		}},
	},
	interopnames.SystemIteratorValue: {
		{"refs_delta", genIteratorValueRefsDelta},
		{"num_bytes", genIteratorValueNumBytes},
	},
	interopnames.SystemRuntimeGetNotifications: {
		{"notifications_count", genGetNotificationsCount},
		{"refs_delta", genGetNotificationsRefsDelta},
	},
	interopnames.SystemRuntimeLoadScript: {
		{"script_len", genLoadScriptScriptLen},
		{"args_count", genLoadScriptArgsCount},
		{"refs_delta", genLoadScriptRefsDelta},
	},
	interopnames.SystemRuntimeNotify: {
		{"events_count", genNotifyEventsCount},
		{"params_count", genNotifyParamsCount},
		{"items_count", genNotifyItemsCount},
	},
}

type depGenerator struct {
	name string
	run  func(t *testing.T, nopNs float64) []sample
}

var staticGenerators = map[string]func(t *testing.T, nopNs float64) float64{
	interopnames.SystemContractCreateStandardAccount: genCreateStandardAccount,
	interopnames.SystemContractGetCallFlags:          genGetCallFlags,
	interopnames.SystemCryptoCheckSig:                genCheckSig,
	interopnames.SystemIteratorNext:                  genIteratorNext,
	interopnames.SystemStorageFind:                   genStorageFindFromDisk,
	interopnames.SystemStorageGet:                    genStorageGetFromDisk,
	interopnames.SystemStorageGetContext:             genStorageGetContext,
	interopnames.SystemStorageGetReadOnlyContext:     genStorageGetReadOnlyContext,
	interopnames.SystemStorageAsReadOnly:             genStorageAsReadOnly,
	interopnames.SystemRuntimeBurnGas:                genBurnGas,
	interopnames.SystemRuntimeGasLeft:                genGasLeft,
	interopnames.SystemRuntimeGetAddressVersion:      genGetAddressVersion,
	interopnames.SystemRuntimeGetCallingScriptHash:   genGetCallingScriptHash,
	interopnames.SystemRuntimeGetEntryScriptHash:     genGetEntryScriptHash,
	interopnames.SystemRuntimeGetExecutingScriptHash: genGetExecutingScriptHash,
	interopnames.SystemRuntimeGetInvocationCounter:   genGetInvocationCounter,
	interopnames.SystemRuntimeGetNetwork:             genGetNetwork,
	interopnames.SystemRuntimeGetRandom:              genGetRandom,
	interopnames.SystemRuntimeGetScriptContainer:     genGetScriptContainer,
	interopnames.SystemRuntimeLog:                    genLog,
	interopnames.SystemRuntimeGetTime:                genGetTime,
	interopnames.SystemRuntimeGetTrigger:             genGetTrigger,
	interopnames.SystemRuntimePlatform:               genPlatform,
}

func combineRows(t *testing.T, deps []depGenerator, nopNs float64) (header []string, rows []sample) {
	for _, dep := range deps {
		header = append(header, dep.name)
	}
	header = append(header, "NOPs")

	for _, dep := range deps {
		rows = append(rows, dep.run(t, nopNs)...)
	}
	return header, rows
}

// sample is a single measurement: dependency values
// and the resulting normalized time.
type sample struct {
	deps  []int
	value float64
}

func writeCSV(dir, name string, header []string, rows []sample) error {
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
	for _, r := range rows {
		record := make([]string, 0, len(r.deps)+1)
		for _, d := range r.deps {
			record = append(record, strconv.Itoa(d))
		}
		record = append(record, strconv.FormatFloat(r.value, 'f', 6, 64))
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func measureNsPerOp(t *testing.T, f func(b *testing.B)) float64 {
	r := testing.Benchmark(f)
	require.NotZero(t, r.N, "benchmark failed")
	return float64(r.NsPerOp())
}

type benchCase struct {
	f            func(*interop.Context) error
	itemsToPush  []any
	numToPop     int
	isStepNeeded bool
}

func benchInterop(b *testing.B, ic *interop.Context, bc benchCase, cs *state.Contract) {
	if ic == nil {
		blkChain, _ := chain.NewSingle(b)
		var err error
		ic, err = blkChain.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})
		require.NoError(b, err)
		ic.VM.LoadScript(nil)
	}
	if cs != nil {
		err := native.PutContractState(ic.DAO, ic.Chain.NativeManagementID(), cs)
		require.NoError(b, err)
	}
	ic.VM.SetGasLimit(-1)

	for b.Loop() {
		b.StopTimer()
		for _, item := range bc.itemsToPush {
			ic.VM.Estack().PushVal(item)
		}
		b.StartTimer()

		err := bc.f(ic)
		b.StopTimer()
		require.NoError(b, err)
		for range bc.numToPop {
			ic.VM.Estack().Pop()
		}
		if bc.isStepNeeded {
			require.NoError(b, ic.VM.Step())
			ic.VM.Estack().Clear()
		}
		b.StartTimer()
	}
}
