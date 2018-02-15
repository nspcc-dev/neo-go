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
)

type testCase struct {
	name   string
	src    string
	result string
}

var testCases = []testCase{
	{
		"simple add",
		`
		package testcase
		func Main() int {
			x := 2 + 2
			return x
		}
		`,
		"52c56b546c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"simple sub",
		`
		package testcase
		func Main() int {
			x := 2 - 2
			return x
		}
		`,
		"52c56b006c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"simple div",
		`
		package testcase
		func Main() int {
			x := 2 / 2
			return x
		}
		`,
		"52c56b516c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"simple mul",
		`
		package testcase
		func Main() int {
			x := 4 * 2
			return x
		}
		`,
		"52c56b586c766b00527ac46203006c766b00c3616c7566",
	},
	{
		"simple binary expr in return",
		`
		package testcase
		func Main() int {
			x := 2
			return 2 + x
		}
		`,
		"52c56b526c766b00527ac4620300526c766b00c393616c7566",
	},
	{
		"complex binary expr",
		`
		package testcase
		func Main() int {
			x := 4
			y := 8
			z := x + 2 + 2 - 8
			return y * z
		}
		`,
		"54c56b546c766b00527ac4586c766b51527ac46c766b00c35293529358946c766b52527ac46203006c766b51c36c766b52c395616c7566",
	},
	{
		"if statement LT",
		`
		package testcase
		func Main() int {
			x := 10
			if x < 100 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c301649f640b0062030051616c756662030000616c7566",
	},
	{
		"if statement GT",
		`
		package testcase
		func Main() int {
			x := 10
			if x > 100 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c30164a0640b0062030051616c756662030000616c7566",
	},
	{
		"if statement GTE",
		`
		package testcase
		func Main() int {
			x := 10
			if x >= 100 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c30164a2640b0062030051616c756662030000616c7566",
	},
	{
		"complex if statement with LAND",
		`
		package testcase
		func Main() int {
			x := 10
			if x >= 10 && x <= 20 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c35aa26416006c766b00c30114a1640b0062030051616c756662030000616c7566",
	},
	{
		"complex if statement with LOR",
		`
		package testcase
		func Main() int {
			x := 10
			if x >= 10 || x <= 20 {
				return 1
			}
			return 0
		}
		`,
		"54c56b5a6c766b00527ac46c766b00c35aa2630e006c766b00c30114a1640b0062030051616c756662030000616c7566",
	},
}

func TestAllCases(t *testing.T) {
	for _, tc := range testCases {
		c := New()
		if err := c.Compile(strings.NewReader(tc.src)); err != nil {
			t.Fatal(err)
		}

		expectedResult, err := hex.DecodeString(tc.result)
		if err != nil {
			t.Fatal(err)
		}

		if bytes.Compare(c.sb.buf.Bytes(), expectedResult) != 0 {
			t.Log(hex.EncodeToString(c.Buffer().Bytes()))
			want, _ := hex.DecodeString(tc.result)
			dumpOpCodeSideBySide(c.Buffer().Bytes(), want)
			t.Fatalf("compiling %s failed", tc.name)
		}
	}
}

func TestSimpleAssign34(t *testing.T) {
	src := `
		package NEP5	

		func Main() int {
			x := 10
			if x < 10 || x < 20 {
				return 1
			}
			return 0
		}
	`

	c := New()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}

	for _, c := range c.funcCalls {
		fmt.Println(c)
	}

	for _, fctx := range c.funcs {
		for _, v := range fctx.scope {
			fmt.Println(v)
		}
	}

	c.DumpOpcode()
}

func dumpOpCodeSideBySide(have, want []byte) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "INDEX\tHAVE OPCODE\tDESC\tWANT OPCODE\tDESC\tDIFF")

	for i := 0; i < len(want); i++ {
		diff := ""
		if have[i] != want[i] {
			diff = "<<"
		}
		fmt.Fprintf(w, "%d\t0x%2x\t%s\t0x%2x\t%s\t%s\n",
			i, have[i], vm.OpCode(have[i]), want[i], vm.OpCode(want[i]), diff)
	}
	w.Flush()
}
