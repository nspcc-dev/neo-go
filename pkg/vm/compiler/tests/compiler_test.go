package compiler_test

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
	testCases = append(testCases, assignTestCases...)
	testCases = append(testCases, arrayTestCases...)
	testCases = append(testCases, functionCallTestCases...)
	testCases = append(testCases, boolTestCases...)
	testCases = append(testCases, stringTestCases...)
	testCases = append(testCases, binaryExprTestCases...)
	testCases = append(testCases, structTestCases...)
	testCases = append(testCases, ifStatementTestCases...)

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

	for i := 0; i < len(have); i++ {
		if len(want) <= i {
			break
		}
		diff := ""
		if have[i] != want[i] {
			diff = "<<"
		}
		fmt.Fprintf(w, "%d\t0x%2x\t%s\t0x%2x\t%s\t%s\n",
			i, have[i], vm.Opcode(have[i]), want[i], vm.Opcode(want[i]), diff)
	}
	w.Flush()
}
