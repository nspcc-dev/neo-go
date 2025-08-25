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

func TestMax(t *testing.T) {
	tests := []struct {
		name        string
		args        []stackitem.Item
		expected    *big.Int
		expectedErr error
		src         string
	}{
		{
			name:        "invalid type named",
			expectedErr: fmt.Errorf("max requires integer types"),
			src: `
				package foo

				type MyInt int
		
				func Main() {
					_ = max(MyInt(1))
				}
			`,
		},
		{
			name:        "invalid type string",
			expectedErr: fmt.Errorf("max requires integer types, got at the 1 position: untyped string"),
			src: `
				package foo
		
				func Main() {
					_ = max("42")
				}
			`,
		},
		{
			name:     "single value",
			args:     []stackitem.Item{stackitem.Make(42)},
			expected: big.NewInt(42),
			src: `
				package foo
		
				func Main(args []any) int64 {
					return max(args[0].(int64))
				}
			`,
		},
		{
			name:     "two values",
			args:     []stackitem.Item{stackitem.Make(10), stackitem.Make(7)},
			expected: big.NewInt(10),
			src: `
				package foo
		
				func Main(args []any) int {
					return max(args[0].(int), args[1].(int))
				}
			`,
		},
		{
			name:     "three values",
			args:     []stackitem.Item{stackitem.Make(5), stackitem.Make(6), stackitem.Make(3)},
			expected: big.NewInt(6),
			src: `
				package foo
		
				func Main(args []any) int {
					return max(args[0].(int), args[1].(int), args[2].(int))
				}
			`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedErr != nil {
				_, _, err := compiler.CompileWithOptions("main.go", strings.NewReader(tc.src), nil)
				require.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				evalWithArgs(t, tc.src, nil, tc.args, tc.expected)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name        string
		args        []stackitem.Item
		expected    *big.Int
		expectedErr error
		src         string
	}{
		{
			name:        "invalid type named",
			expectedErr: fmt.Errorf("min requires integer types"),
			src: `
				package foo

				type MyInt int
		
				func Main() {
					_ = min(MyInt(1))
				}
			`,
		},
		{
			name:        "invalid type string",
			expectedErr: fmt.Errorf("min requires integer types, got at the 1 position: untyped string"),
			src: `
				package foo
		
				func Main() {
					_ = min("42")
				}
			`,
		},
		{
			name:     "single value",
			args:     []stackitem.Item{stackitem.Make(42)},
			expected: big.NewInt(42),
			src: `
				package foo
		
				func Main(args []any) int64 {
					return min(args[0].(int64))
				}
			`,
		},
		{
			name:     "two values",
			args:     []stackitem.Item{stackitem.Make(10), stackitem.Make(7)},
			expected: big.NewInt(7),
			src: `
				package foo
		
				func Main(args []any) int {
					return min(args[0].(int), args[1].(int))
				}
			`,
		},
		{
			name:     "three values",
			args:     []stackitem.Item{stackitem.Make(5), stackitem.Make(6), stackitem.Make(3)},
			expected: big.NewInt(3),
			src: `
				package foo
		
				func Main(args []any) int {
					return min(args[0].(int), args[1].(int), args[2].(int))
				}
			`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedErr != nil {
				_, _, err := compiler.CompileWithOptions("main.go", strings.NewReader(tc.src), nil)
				require.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				evalWithArgs(t, tc.src, nil, tc.args, tc.expected)
			}
		})
	}
}
