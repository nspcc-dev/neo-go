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
	testCases = append(testCases, stringTestCases...)
	testCases = append(testCases, binaryExprTestCases...)
	testCases = append(testCases, ifStatementTestCases...)
	testCases = append(testCases, functionCallTestCases...)

	for _, tc := range testCases {
		c := compiler.New()
		if err := c.Compile(strings.NewReader(tc.src)); err != nil {
			t.Fatal(err)
		}

		expectedResult, err := hex.DecodeString(tc.result)
		if err != nil {
			t.Fatal(err)
		}

		if bytes.Compare(c.Buffer().Bytes(), expectedResult) != 0 {
			t.Log(hex.EncodeToString(c.Buffer().Bytes()))
			want, _ := hex.DecodeString(tc.result)
			dumpOpCodeSideBySide(c.Buffer().Bytes(), want)
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
			i, have[i], vm.OpCode(have[i]), want[i], vm.OpCode(want[i]), diff)
	}
	w.Flush()
}
