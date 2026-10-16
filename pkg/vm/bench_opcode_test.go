package vm

import (
	"encoding/csv"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

const (
	// generatePrices enables price CSV generation. To regenerate files in
	// genOutDir, set it to true, run TestGenerateOpcodePriceCSV and
	// set it back to false.
	generatePrices = false
	// genOutDir is a directory price CSV files are written to.
	genOutDir = "testdata/prices"
	// nopCount is the number of NOPs in the script NOP time is measured with.
	nopCount = 10000
)

func TestGenerateOpcodePriceCSV(t *testing.T) {
	if generatePrices {
		nopNs := measureNsPerOp(t, benchmarkNOP) / nopCount
		require.NoError(t, writeCSV(genOutDir, opcode.MODPOW.String(),
			[]string{"length", "NOPs"}, genMODPOW(t, nopNs)))
	}
	require.False(t, generatePrices)
}

func benchmarkNOP(b *testing.B) {
	script := make([]byte, nopCount+1)
	for i := range nopCount {
		script[i] = byte(opcode.NOP)
	}
	script[nopCount] = byte(opcode.RET)
	v := New()
	for b.Loop() {
		b.StopTimer()
		v.LoadScript(script)
		b.StartTimer()
		require.NoError(b, v.Run())
	}
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

type benchOpcodeCase struct {
	op       opcode.Opcode
	items    []stackitem.Item
	numToPop int
}

func benchOpcodeLoop(b *testing.B, c benchOpcodeCase) {
	v := load([]byte{byte(c.op)})
	ctx := v.Context()
	for b.Loop() {
		b.StopTimer()
		for _, item := range c.items {
			v.estack.PushItem(copyItem(item))
		}
		b.StartTimer()
		_, err := v.execute(ctx, c.op, nil)
		b.StopTimer()
		require.NoError(b, err)
		for range c.numToPop {
			v.estack.Pop()
		}
		b.StartTimer()
	}
}

func maxIntOfBytes(numBytes int) stackitem.Item {
	x := new(big.Int).Lsh(big.NewInt(1), uint(8*numBytes-1))
	return stackitem.NewBigInteger(x.Sub(x, big.NewInt(1)))
}

func genMODPOW(t *testing.T, nopNs float64) []sample {
	const maxBytes = stackitem.MaxBigIntegerSizeBits / 8
	base := stackitem.NewBigInteger(new(big.Int).Exp(big.NewInt(3), big.NewInt(160), nil))
	exp := maxIntOfBytes(maxBytes)
	measure := func(modBytes int) sample {
		items := []stackitem.Item{base, exp, maxIntOfBytes(modBytes)}
		ns := measureNsPerOp(t, func(b *testing.B) {
			benchOpcodeLoop(b, benchOpcodeCase{
				op:       opcode.MODPOW,
				items:    items,
				numToPop: 1,
			})
		})
		return sample{deps: []int{modBytes}, value: ns / nopNs}
	}

	rows := make([]sample, 0, countPoints+1)
	rows = append(rows, measure(1))
	step := maxBytes / countPoints
	curr := step
	for range countPoints {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}
