package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

var assignTestCases = []testCase{
	{
		"chain define",
		`func F%d() int {
			x := 4
			y := x
			z := y
			foo := z
			bar := foo
			return bar
		}
		`,
		big.NewInt(4),
	},
	{
		"simple assign",
		`func F%d() int {
			x := 4
			x = 8
			return x
		}
		`,
		big.NewInt(8),
	},
	{
		"add assign",
		`func F%d() int {
			x := 4
			x += 8
			return x
		}
		`,
		big.NewInt(12),
	},
	{
		"sub assign",
		`func F%d() int {
			x := 4
			x -= 2
			return x
		}
		`,
		big.NewInt(2),
	},
	{
		"mul assign",
		`func F%d() int {
			x := 4
			x *= 2
			return x
		}
		`,
		big.NewInt(8),
	},
	{
		"div assign",
		`func F%d() int {
			x := 4
			x /= 2
			return x
		}
		`,
		big.NewInt(2),
	},
	{
		"add assign binary expr",
		`func F%d() int {
			x := 4
			x += 6 + 2
			return x
		}
		`,
		big.NewInt(12),
	},
	{
		"add assign binary expr ident",
		`func F%d() int {
			x := 4
			y := 5
			x += 6 + y
			return x
		}
		`,
		big.NewInt(15),
	},
	{
		"add assign for string",
		`func F%d() string {
			s := "Hello, "
			s += "world!"
			return s
		}
		`,
		[]byte("Hello, world!"),
	},
	{
		"decl assign",
		`func F%d() int {
			var x int = 4
			return x
		}
		`,
		big.NewInt(4),
	},
	{
		"multi assign",
		`func F%d() int {
			x, y := 1, 2
			return x + y
		}
		`,
		big.NewInt(3),
	},
	{
		"many assignments",
		`func F%d() int {
			a := 0
			` + strings.Repeat("a += 1\n", 1024) + `
			return a
		}
		`,
		big.NewInt(1024),
	},
}

func TestAssignments(t *testing.T) {
	srcBuilder := bytes.NewBuffer([]byte("package testcase\n"))
	for i, tc := range assignTestCases {
		srcBuilder.WriteString(fmt.Sprintf(tc.src, i))
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	for i, tc := range assignTestCases {
		v := vm.New()
		t.Run(tc.name, func(t *testing.T) {
			v.Reset(trigger.Application)
			invokeMethod(t, fmt.Sprintf("F%d", i), ne.Script, v, di)
			runAndCheck(t, v, tc.result)
		})
	}
}
