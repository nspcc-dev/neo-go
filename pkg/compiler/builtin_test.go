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

func TestClear(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		expectedErr string
		expected    any
	}{
		{
			name: "invalid type array",
			src: `package foo
				func Main() [42]int {
					var m = [42]int{}
					clear(m)
					return m
				}`,
			expectedErr: "invalid argument: cannot clear m (variable of type [42]int): " +
				"argument must be (or constrained by) map or slice",
		},
		{
			name: "map",
			src: `package foo
				func Main() map[string]int {
					var m = map[string]int{
						"one": 1,
						"two": 2,
					}
					clear(m)
					return m
				}`,
			expected: []stackitem.MapElement{},
		},
		{
			name: "slice of integers",
			src: `package foo
				func Main() []int {
					var s = []int{1, 2, 3}
					clear(s)
					return s
				}`,
			expected: []stackitem.Item{stackitem.Make(0), stackitem.Make(0), stackitem.Make(0)},
		},
		{
			name: "slice of strings",
			src: `package foo
				func Main() []string {
					var s = []string{"one", "two", "three"}
					clear(s)
					return s
				}`,
			expected: []stackitem.Item{stackitem.Make(""), stackitem.Make(""), stackitem.Make("")},
		},
		{
			name: "slice of structs",
			src: `package foo
				type S2 struct {
					A int
					B string
					C bool
				}
					
				type S1 struct {
					S S2
				}
				
				func Main() []S1 {
					var s = []S1 {
						{
							S: S2 {
								A: 42,
								B: "first",
								C: true,
							},
						},
						{
							S: S2 {
								A: 24,
								B: "second",
								C: true,
							},
						},
					}
					clear(s)
					return s
				}`,
			expected: []stackitem.Item{
				stackitem.NewStruct([]stackitem.Item{
					stackitem.NewStruct([]stackitem.Item{
						stackitem.Make(0),
						stackitem.Make(""),
						stackitem.Make(false),
					}),
				}),
				stackitem.NewStruct([]stackitem.Item{
					stackitem.NewStruct([]stackitem.Item{
						stackitem.Make(0),
						stackitem.Make(""),
						stackitem.Make(false),
					}),
				}),
			},
		},
		{
			name: "slice of slices",
			src: `package foo
				func Main() [][]int {
					var s = [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8}}
					clear(s)
					return s
				}`,
			expected: []stackitem.Item{stackitem.Null{}, stackitem.Null{}, stackitem.Null{}},
		},
		{
			name: "slice of maps",
			src: `package foo
				func Main() []map[string]string {
					var s = []map[string]string{
						{
							"key": "val",
						},
						{
							"key": "val",
						},
					}
					clear(s)
					return s
				}`,
			expected: []stackitem.Item{stackitem.Null{}, stackitem.Null{}},
		},
		{
			name: "empty slice",
			src: `package foo
				func Main() []any {
					var s = []any{}
					clear(s)
					return s
				}`,
			expected: []stackitem.Item{},
		},
		{
			name: "empty map",
			src: `package foo
				func Main() map[any]any {
					var m = map[any]any{}
					clear(m)
					return m
				}`,
			expected: []stackitem.MapElement{},
		},
		{
			name: "slice of simple structs",
			src: `package foo
				type S struct {
					A int
				}
				
				func Main() []S {
					var s = []S {
						{
							A: 1,	
						},
						{
							A: 2,
						},
					}
					clear(s)
					s[0].A = 42
					return s
				}`,
			expected: []stackitem.Item{
				stackitem.NewStruct([]stackitem.Item{
					stackitem.Make(42),
				}),
				stackitem.NewStruct([]stackitem.Item{
					stackitem.Make(0),
				}),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedErr != "" {
				_, _, err := compiler.CompileWithOptions("main.go", strings.NewReader(tc.src), nil)
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				eval(t, tc.src, tc.expected)
			}
		})
	}
}
