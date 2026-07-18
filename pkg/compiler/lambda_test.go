package compiler_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/stretchr/testify/require"
)

func TestFuncLiteral(t *testing.T) {
	src := `package foo
	func Main() int {
		inc := func(x int) int { return x + 1 }
		return inc(1) + inc(2)
	}`
	eval(t, src, big.NewInt(5))
}

func TestCallInPlace(t *testing.T) {
	src := `package foo
	var a int = 1
	func Main() int {
		func() {
			a += 10
		}()
		a += 100
		return a
	}`
	eval(t, src, big.NewInt(111))
}

func TestImmediatelyInvokedFuncLitReturningFunc(t *testing.T) {
	src := `package foo
	func Main() int {
		f := func() func() int {
			return func() int { return 42 }
		}
		return f()()
	}`
	eval(t, src, big.NewInt(42))
}

func TestLambdaInDebugInfo(t *testing.T) {
	testCases := []struct {
		name      string
		src       string
		seqPoints map[string][]compiler.DebugSeqPoint
	}{
		{
			name: "simple",
			src: `package main
				import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
				func PublicContractMethod() {
					var f = func() {
						runtime.Log("bla")
					}

					f()
					f()
				}
			`,
			seqPoints: map[string][]compiler.DebugSeqPoint{
				"PublicContractMethod": {
					{StartLine: 4, StartCol: 14, EndLine: 4, EndCol: 20},
					{StartLine: 4, StartCol: 10, EndLine: 4, EndCol: 14},
					{StartLine: 8, StartCol: 6, EndLine: 8, EndCol: 9},
					{StartLine: 9, StartCol: 6, EndLine: 9, EndCol: 9},
				},
				"lambda@56": {
					{StartLine: 5, StartCol: 7, EndLine: 5, EndCol: 25},
					{StartLine: 60, StartCol: 2, EndLine: 60, EndCol: 63},
				},
			},
		},
		{
			name: "multiple lambdas",
			src: `package main
				import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
				func PublicContractMethod() {
					f1 := func() {
						runtime.Log("1")
					}
					f2 := func() {
						runtime.Log("2")
					}
					f1()
					f2()
				}
			`,
			seqPoints: map[string][]compiler.DebugSeqPoint{
				"PublicContractMethod": {
					{StartLine: 4, StartCol: 12, EndLine: 4, EndCol: 18},
					{StartLine: 4, StartCol: 6, EndLine: 4, EndCol: 12},
					{StartLine: 7, StartCol: 12, EndLine: 7, EndCol: 18},
					{StartLine: 7, StartCol: 6, EndLine: 7, EndCol: 12},
					{StartLine: 10, StartCol: 6, EndLine: 10, EndCol: 10},
					{StartLine: 11, StartCol: 6, EndLine: 11, EndCol: 10},
				},
				"lambda@56": {
					{StartLine: 5, StartCol: 7, EndLine: 5, EndCol: 23},
					{StartLine: 60, StartCol: 2, EndLine: 60, EndCol: 63},
				},
				"lambda@57": {
					{StartLine: 8, StartCol: 7, EndLine: 8, EndCol: 23},
					{StartLine: 60, StartCol: 2, EndLine: 60, EndCol: 63},
				},
			},
		},
		{
			name: "nested lambdas",
			src: `package main
				import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
				func PublicContractMethod() {
					f := func() {
						g := func() {
							runtime.Log("nested")
						}
						g()
					}
					f()
				}
			`,
			seqPoints: map[string][]compiler.DebugSeqPoint{
				"PublicContractMethod": {
					{StartLine: 4, StartCol: 11, EndLine: 4, EndCol: 17},
					{StartLine: 4, StartCol: 6, EndLine: 4, EndCol: 11},
					{StartLine: 10, StartCol: 6, EndLine: 10, EndCol: 9},
				},
				"lambda@56": {
					{StartLine: 5, StartCol: 12, EndLine: 5, EndCol: 18},
					{StartLine: 5, StartCol: 7, EndLine: 5, EndCol: 12},
					{StartLine: 8, StartCol: 7, EndLine: 8, EndCol: 10},
				},
				"lambda@57": {
					{StartLine: 6, StartCol: 8, EndLine: 6, EndCol: 29},
					{StartLine: 60, StartCol: 2, EndLine: 60, EndCol: 63},
				},
			},
		},
		{
			name: "lambda returns lambda",
			src: `package main
				import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
				func PublicContractMethod() {
					f := func() func() {
						return func() {
							runtime.Log("inner")
						}
					}
					g := f()
					g()
				}
			`,
			seqPoints: map[string][]compiler.DebugSeqPoint{
				"PublicContractMethod": {
					{StartLine: 4, StartCol: 11, EndLine: 4, EndCol: 24},
					{StartLine: 4, StartCol: 6, EndLine: 4, EndCol: 11},
					{StartLine: 9, StartCol: 11, EndLine: 9, EndCol: 14},
					{StartLine: 9, StartCol: 6, EndLine: 9, EndCol: 11},
					{StartLine: 10, StartCol: 6, EndLine: 10, EndCol: 9},
				},
				"lambda@56": {
					{StartLine: 5, StartCol: 14, EndLine: 5, EndCol: 20},
					{StartLine: 5, StartCol: 7, EndLine: 5, EndCol: 13},
				},
				"lambda@58": {
					{StartLine: 6, StartCol: 8, EndLine: 6, EndCol: 28},
					{StartLine: 60, StartCol: 2, EndLine: 60, EndCol: 63},
				},
			},
		},
		{
			name: "three-level nested lambdas",
			src: `package main
				import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
				func PublicContractMethod() {
					f := func() {
						g := func() {
							h := func() {
								runtime.Log("deep")
							}
							h()
						}
						g()
					}
					f()
				}
			`,
			seqPoints: map[string][]compiler.DebugSeqPoint{
				"PublicContractMethod": {
					{StartLine: 4, StartCol: 11, EndLine: 4, EndCol: 17},
					{StartLine: 4, StartCol: 6, EndLine: 4, EndCol: 11},
					{StartLine: 13, StartCol: 6, EndLine: 13, EndCol: 9},
				},
				"lambda@56": {
					{StartLine: 5, StartCol: 12, EndLine: 5, EndCol: 18},
					{StartLine: 5, StartCol: 7, EndLine: 5, EndCol: 12},
					{StartLine: 11, StartCol: 7, EndLine: 11, EndCol: 10},
				},
				"lambda@57": {
					{StartLine: 6, StartCol: 13, EndLine: 6, EndCol: 19},
					{StartLine: 6, StartCol: 8, EndLine: 6, EndCol: 13},
					{StartLine: 9, StartCol: 8, EndLine: 9, EndCol: 11},
				},
				"lambda@58": {
					{StartLine: 7, StartCol: 9, EndLine: 7, EndCol: 28},
					{StartLine: 60, StartCol: 2, EndLine: 60, EndCol: 63},
				},
			},
		},
		{
			name: "lambda returns lambda returns lambda",
			src: `package main
				import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
				func PublicContractMethod() {
					f := func() func() func() {
						return func() func() {
							return func() {
								runtime.Log("deep")
							}
						}
					}
					g := f()
					h := g()
					h()
				}
			`,
			seqPoints: map[string][]compiler.DebugSeqPoint{
				"PublicContractMethod": {
					{StartLine: 4, StartCol: 11, EndLine: 4, EndCol: 31},
					{StartLine: 4, StartCol: 6, EndLine: 4, EndCol: 11},
					{StartLine: 11, StartCol: 11, EndLine: 11, EndCol: 14},
					{StartLine: 11, StartCol: 6, EndLine: 11, EndCol: 11},
					{StartLine: 12, StartCol: 11, EndLine: 12, EndCol: 14},
					{StartLine: 12, StartCol: 6, EndLine: 12, EndCol: 11},
					{StartLine: 13, StartCol: 6, EndLine: 13, EndCol: 9},
				},
				"lambda@56": {
					{StartLine: 5, StartCol: 14, EndLine: 5, EndCol: 27},
					{StartLine: 5, StartCol: 7, EndLine: 5, EndCol: 13},
				},
				"lambda@58": {
					{StartLine: 6, StartCol: 15, EndLine: 6, EndCol: 21},
					{StartLine: 6, StartCol: 8, EndLine: 6, EndCol: 14},
				},
				"lambda@60": {
					{StartLine: 7, StartCol: 9, EndLine: 7, EndCol: 28},
					{StartLine: 60, StartCol: 2, EndLine: 60, EndCol: 63},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, di, err := compiler.CompileWithOptions("test.go", strings.NewReader(tc.src), nil)
			require.NoError(t, err)
			for _, m := range di.Methods {
				expected, ok := tc.seqPoints[m.ID]
				require.True(t, ok)
				require.Len(t, m.SeqPoints, len(expected))
				for i := range m.SeqPoints {
					require.Equal(t, expected[i].StartLine, m.SeqPoints[i].StartLine)
					require.Equal(t, expected[i].StartCol, m.SeqPoints[i].StartCol)
					require.Equal(t, expected[i].EndLine, m.SeqPoints[i].EndLine)
					require.Equal(t, expected[i].EndCol, m.SeqPoints[i].EndCol)
				}
			}
		})
	}
}
