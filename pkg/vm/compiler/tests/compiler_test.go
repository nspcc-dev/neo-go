package compiler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
)

type testCase struct {
	name   string
	src    string
	result string
}

func TestAllCases(t *testing.T) {
	testCases := []testCase{}

	// The Go language
	testCases = append(testCases, assignTestCases...)
	testCases = append(testCases, arrayTestCases...)
	testCases = append(testCases, binaryExprTestCases...)
	testCases = append(testCases, functionCallTestCases...)
	testCases = append(testCases, boolTestCases...)
	testCases = append(testCases, stringTestCases...)
	testCases = append(testCases, structTestCases...)
	testCases = append(testCases, ifStatementTestCases...)
	testCases = append(testCases, customTypeTestCases...)
	testCases = append(testCases, constantTestCases...)

	// TODO: issue #28
	// These tests are passing locally, but circleci is failing to resolve the dependency.
	// https://github.com/CityOfZion/neo-go/issues/28
	testCases = append(testCases, importTestCases...)

	// Blockchain specific
	testCases = append(testCases, storageTestCases...)

	for _, tc := range testCases {
		b, err := compiler.Compile(strings.NewReader(tc.src), &compiler.Options{})
		if err != nil {
			t.Fatal(err)
		}

		expectedResult, err := hex.DecodeString(tc.result)
		if err != nil {
			t.Fatal(err)
		}

		if bytes.Compare(b, expectedResult) != 0 {
			t.Log(hex.EncodeToString(b))
			want, _ := hex.DecodeString(tc.result)
			dumpOpCodeSideBySide(b, want)
			t.Fatalf("compiling %s failed", tc.name)
		}
	}
}

func dumpOpCodeSideBySide(have, want []byte) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "INDEX\tHAVE OPCODE\tDESC\tWANT OPCODE\tDESC\tDIFF")

	var b byte
	for i := 0; i < len(have); i++ {
		if len(want) <= i {
			b = 0x00
		} else {
			b = want[i]
		}
		diff := ""
		if have[i] != b {
			diff = "<<"
		}
		fmt.Fprintf(w, "%d\t0x%2x\t%s\t0x%2x\t%s\t%s\n",
			i, have[i], vm.Opcode(have[i]), b, vm.Opcode(b), diff)
	}
	w.Flush()
}
