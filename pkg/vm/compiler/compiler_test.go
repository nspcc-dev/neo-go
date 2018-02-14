package compiler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
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
			t.Fatalf("compiling %s failed", tc.name)
		}
	}
}

func TestSimpleAssign34(t *testing.T) {
	src := `
		package NEP5	

		func Main() int {
			x := 10
			if x < 100 {
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
		fmt.Println(fctx.label)
		for _, v := range fctx.scope {
			fmt.Println(v)
		}
	}

	c.DumpOpcode()
}
