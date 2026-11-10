package benchmarks

import (
	"encoding/csv"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

const (
	// genOutDir is a directory price CSV files are written to.
	genOutDir = "nativefee/benchmark_results"
	// nopCount is the number of NOPs in the script NOP time is measured with.
	nopCount = 10000
)

var (
	generatePrices  = flag.Bool("generate-prices", false, "Generate native contract pricing CSV files")
	benchmarkFilter = flag.String("benchmarks", ".*", "Regular expression matching native benchmark names")
)

func TestGenerateNativePriceCSV(t *testing.T) {
	if !*generatePrices {
		t.Skip("use -generate-prices to generate native pricing CSV files")
	}

	filter, err := regexp.Compile(*benchmarkFilter)
	require.NoError(t, err, "invalid benchmark filter")
	names := make([]string, 0, len(nativeGenerators)+len(staticGenerators))
	for name := range nativeGenerators {
		if filter.MatchString(name) {
			names = append(names, name)
		}
	}
	for name := range staticGenerators {
		if filter.MatchString(name) {
			names = append(names, name)
		}
	}
	require.NotEmpty(t, names, "no benchmarks match %q", *benchmarkFilter)
	sort.Strings(names)

	bc := newPriceChain(t)
	nopIC := newPriceContext(t, bc, nil)
	nopNs := measureNsPerOp(t, func(b *testing.B) { benchmarkNOP(b, nopIC) }) / nopCount
	for _, name := range names {
		if deps, ok := nativeGenerators[name]; ok {
			header, rows := combineRows(t, deps, nopNs)
			require.NoError(t, writeCSV(genOutDir, name, header, rows))
			continue
		}
		require.NoError(t, writeCSV(genOutDir, name, []string{"nops"}, []sample{{value: staticGenerators[name](t, nopNs)}}))
	}
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

var nativeGenerators = map[string][]depGenerator{
	nativenames.Management + ".GetContract": {
		{"nefsize", genManagementGetContractNEFSize},
		{"refsdelta", genManagementGetContractRefsDelta},
		{"methodnamelen", genManagementGetContractMethodNameLen},
	},
	nativenames.Management + ".HasMethod": {
		{"methodcount", genManagementHasMethodCount},
		{"methodnamelen", genManagementHasMethodNameLen},
	},
	nativenames.Management + ".Update": {
		{"tokenscount", genManagementTokensCount},
		{"scriptlen", genManagementScriptLen},
		{"manifestsize", genManagementManifestSize},
		{"groupcount", genManagementGroupCount},
		{"permissioncount", genManagementPermissionCount},
		{"standardcount", genManagementStandardCount},
		{"trustcount", genManagementTrustCount},
		{"methodcount", genManagementMethodCount},
		{"eventcount", genManagementEventCount},
		{"paramcount", genManagementParamCount},
	},
}

type depGenerator struct {
	name string
	run  func(t *testing.T, nopNs float64) []sample
}

var staticGenerators = map[string]func(t *testing.T, nopNs float64) float64{
	nativenames.Management + ".Destroy":                 genManagementDestroy,
	nativenames.Management + ".GetMinimumDeploymentFee": genManagementGetMinimumDeploymentFee,
	nativenames.Management + ".IsContract":              genManagementIsContract,
	nativenames.Management + ".SetMinimumDeploymentFee": genManagementSetMinimumDeploymentFee,
}

func combineRows(t *testing.T, deps []depGenerator, nopNs float64) (header []string, rows []sample) {
	for _, dep := range deps {
		header = append(header, dep.name)
	}
	header = append(header, "nops")

	for _, dep := range deps {
		rows = append(rows, dep.run(t, nopNs)...)
	}
	return header, rows
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
