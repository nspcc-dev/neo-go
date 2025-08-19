package compiler_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestMaxMin(t *testing.T) {
	tests := []struct {
		name        string
		args        []stackitem.Item
		expected    map[string]*big.Int
		expectedErr string
		srcTemplate string
	}{
		{
			name:        "invalid type named",
			expectedErr: "only integer types are supported by %s",
			srcTemplate: `
				package foo

				type MyInt int

				func Main() {
					_ = %s(MyInt(1))
				}
			`,
		},
		{
			name:        "invalid type string",
			expectedErr: "only integer types are supported by %s, got untyped string at 1",
			srcTemplate: `
				package foo

				func Main() {
					_ = %s("42")
				}
			`,
		},
		{
			name: "single value as func call",
			expected: map[string]*big.Int{
				"max": big.NewInt(42),
				"min": big.NewInt(42),
			},
			srcTemplate: `
				package foo

				func f() int {
					return 42
				}

				func Main() int {
					return %s(f())
				}
			`,
		},
		{
			name: "two values",
			args: []stackitem.Item{stackitem.Make(10), stackitem.Make(7)},
			expected: map[string]*big.Int{
				"max": big.NewInt(10),
				"min": big.NewInt(7),
			},
			srcTemplate: `
				package foo

				func Main(args []any) int {
					return %s(args[0].(int), args[1].(int))
				}
			`,
		},
		{
			name: "three values",
			args: []stackitem.Item{stackitem.Make(5), stackitem.Make(6), stackitem.Make(3)},
			expected: map[string]*big.Int{
				"max": big.NewInt(6),
				"min": big.NewInt(3),
			},
			srcTemplate: `
				package foo

				func Main(args []any) int {
					return %s(args[0].(int), args[1].(int), args[2].(int))
				}
			`,
		},
	}

	for _, op := range []string{"max", "min"} {
		for _, tc := range tests {
			src := fmt.Sprintf(tc.srcTemplate, op)
			if tc.expectedErr != "" {
				_, _, err := compiler.CompileWithOptions("main.go", strings.NewReader(src), nil)
				require.ErrorContains(t, err, fmt.Sprintf(tc.expectedErr, op))
			} else {
				evalWithArgs(t, src, nil, tc.args, tc.expected[op])
			}
		}
	}
}
