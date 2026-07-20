package compiler_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestArray(t *testing.T) {
	testCases := []testCase{
		{
			"constant index",
			`package foo
			func Main() int {
				a := [2]int{}
				a[1] = 42
				return a[1]
			}`,
			big.NewInt(42),
		},
		{
			"variable index",
			`package foo
			func Main() int {
				a := [2]int{}
				i := 1
				a[i] = 42
				return a[1]
			}`,
			big.NewInt(42),
		},
		{
			"declared with var, no initializer",
			`package foo
			func Main() int {
				var a [3]int
				a[2] = 42
				return a[0] + a[1] + a[2]
			}`,
			big.NewInt(42),
		},
		{
			"positional literal",
			`package foo
			func Main() int {
				a := [3]int{1, 2, 3}
				return a[0] + a[1] + a[2]
			}`,
			big.NewInt(6),
		},
		{
			"keyed literal with gaps",
			`package foo
			func Main() int {
				a := [4]int{2: 40, 3: 2}
				return a[0] + a[1] + a[2] + a[3]
			}`,
			big.NewInt(42),
		},
		{
			"elided length",
			`package foo
			func Main() int {
				a := [...]int{1, 2, 3}
				return len(a) + a[0] + a[1] + a[2]
			}`,
			big.NewInt(9),
		},
		{
			"len of array",
			`package foo
			func Main() int {
				var a [5]int
				return len(a)
			}`,
			big.NewInt(5),
		},
		{
			"range over array",
			`package foo
			func Main() int {
				a := [3]int{1, 2, 3}
				sum := 0
				for _, v := range a {
					sum += v
				}
				return sum
			}`,
			big.NewInt(6),
		},
		{
			"range value variable is a copy",
			`package foo
			func Main() int {
				nested := [][2]int{{40, 2}, {1: 2}}
				for _, arr := range nested {
					arr[0] = -1
					arr[1] = -1
				}
				return nested[0][0] + nested[1][1]
			}`,
			big.NewInt(42),
		},
		{
			"array of structs",
			`package foo
			type Point struct { X, Y int }
			func Main() int {
				a := [2]Point{{1, 2}, {3, 4}}
				return a[0].Y + a[1].X
			}`,
			big.NewInt(5),
		},
		{
			"array of pointers to structs",
			`package foo
			type Point struct { X, Y int }
			func Main() int {
				arr1 := [2]*Point{{X: 1, Y: 2}, {X: 3, Y: 4}}
				arr2 := arr1
				arr2[0].X = 0
				arr2[1].Y = 42
				return arr1[0].X + arr1[1].Y
			}`,
			big.NewInt(42),
		},
		{
			"struct field of array type",
			`package foo
			type S struct { Arr [3]int }
			func Main() int {
				var s S
				s.Arr[1] = 42
				return s.Arr[0] + s.Arr[1] + s.Arr[2]
			}`,
			big.NewInt(42),
		},
		{
			"array is copied to function call",
			`package foo
			func mutate(arr [4]int) {
				arr[3] = -1
			}
			func Main() int {
				arr := [4]int{3: 42}
				mutate(arr)
				return arr[3]
			}`,
			big.NewInt(42),
		},
		{
			"assigning an array copies it",
			`package foo
			func Main() int {
				a := [3]int{1, 2, 3}
				b := a
				b[0] = 99
				return a[0]
			}`,
			big.NewInt(1),
		},
		{
			"nested array",
			`package foo
			func Main() int {
				arr1 := [2][2]int{{40, 2}, {3, 2}}
				arr2 := arr1
				arr2[0][0] = -1
				arr2[1][1] = -1
				return arr1[0][0] + arr1[1][1]
			}`,
			big.NewInt(42),
		},
		{
			"named array type",
			`package foo
			type Arr4 [4]int
			func Main() int {
				a := Arr4{1, 2, 3, 4}
				return a[2]
			}`,
			big.NewInt(3),
		},
		{
			"array of a named slice type",
			`package foo
			type MyInts []int
			func Main() int {
				arr1 := [2]MyInts{{1, 2}, {3, 4, 5}}
				arr2 := arr1
				return arr2[0][0] + arr2[1][0] + arr2[1][2]
			}`,
			big.NewInt(9),
		},
		{
			"copy array with ok flag check",
			`package foo
			func Main() int {
				m := map[string][4]int{"key": {3: 42}}
				arr, _ := m["key"]
				arr[3] = -1
				return m["key"][3]
			}`,
			big.NewInt(42),
		},
		{
			"array receiver copy semantics",
			`package foo
			type Row [1]int
			func (r Row) DontMutate() Row { r[0] = -1; return r }
			func (r *Row) Mutate() *Row { r[0] = -1; return r }
			func Main() []int {
				r := Row{42}
				res1 := r.DontMutate()[0]
				res2 := r[0]
				res3 := r.Mutate()[0]
				res4 := r[0]
				return []int{res1, res2, res3, res4}
			}`,
			[]stackitem.Item{stackitem.Make(-1), stackitem.Make(42), stackitem.Make(-1), stackitem.Make(-1)},
		},
		{
			"type assertion from any to array",
			`package foo
			func Main() int {
				var arr any = [1]int{42}
				return arr.([1]int)[0]
			}`,
			big.NewInt(42),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) { eval(t, tc.src, tc.result) })
	}
}

func TestByteArray(t *testing.T) {
	testCases := []testCase{
		{
			"byte array literal and index",
			`package foo
			func Main() int {
				a := [3]byte{1, 2, 3}
				a[1] = 42
				return int(a[1])
			}`,
			big.NewInt(42),
		},
		{
			"byte array with variable element",
			`package foo
			func Main() byte {
				x := byte(5)
				a := [2]byte{x, 6}
				return a[0] + a[1]
			}`,
			big.NewInt(11),
		},
		{
			"byte array keyed literal with gaps",
			`package foo
			func Main() byte {
				a := [4]byte{0: 20, 2: 22}
				return a[0] + a[1] + a[2] + a[3]
			}`,
			big.NewInt(42),
		},
		{
			"byte array declared with var, no initializer",
			`package foo
			func Main() int {
				var a [4]byte
				return len(a) + int(a[0])
			}`,
			big.NewInt(4),
		},
		{
			"len of byte array",
			`package foo
			func Main() int {
				var a [5]byte
				return len(a)
			}`,
			big.NewInt(5),
		},
		{
			"range over byte array",
			`package foo
			func Main() byte {
				a := [3]byte{1, 2, 3}
				sum := byte(0)
				for _, v := range a {
					sum += v
				}
				return sum
			}`,
			big.NewInt(6),
		},
		{
			"named byte array type",
			`package foo
			type Hash4 [4]byte
			func Main() byte {
				h := Hash4{1, 2, 3, 42}
				return h[3]
			}`,
			big.NewInt(42),
		},
		{
			"assigning a byte array copies the underlying buffer",
			`package foo
			func Main() byte {
				a := [3]byte{42, 2, 3}
				b := a
				b[0] = 0
				return a[0]
			}`,
			big.NewInt(42),
		},
		{
			"type assertion from any to byte array",
			`package foo
			func Main() byte {
				var arr any = [1]byte{42}
				return arr.([1]byte)[0]
			}`,
			big.NewInt(42),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) { eval(t, tc.src, tc.result) })
	}
}

func TestArrayConversion(t *testing.T) {
	testCases := []testCase{
		{
			"slice to array",
			`package foo
			func Main() int {
				s := []int{1, 2, 3, 4}
				a := [4]int(s)
				return a[0] + a[1] + a[2] + a[3]
			}`,
			big.NewInt(10),
		},
		{
			"slice to array truncates extra elements",
			`package foo
			func Main() int {
				s := []int{1, 2, 3, 4, 5}
				a := [4]int(s)
				return a[0] + a[3] + len(a)
			}`,
			big.NewInt(9),
		},
		{
			"byte slice to byte array",
			`package foo
			func Main() int {
				s := []byte{1, 2, 3, 4}
				a := [4]byte(s)
				return int(a[3])
			}`,
			big.NewInt(4),
		},
		{
			"named slice to array via literal array type",
			`package foo
			type MyInts []int
			func Main() int {
				s := MyInts{1, 2, 3, 4}
				a := [4]int(s)
				return a[3]
			}`,
			big.NewInt(4),
		},
		{
			"conversion via a named array type truncates just like the literal form",
			`package foo
			type MyInts []int
			type Arr2 [2]int
			func Main() int {
				s := MyInts{1, 2, 3, 4}
				a := Arr2(s)
				return len(a)
			}`,
			big.NewInt(2),
		},
		{
			"conversion via a named array type makes an independent copy",
			`package foo
			type Arr4 [4]int
			func Main() int {
				a := [4]int{1, 2, 3, 4}
				b := Arr4(a)
				b[0] = 99
				return a[0]
			}`,
			big.NewInt(1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) { eval(t, tc.src, tc.result) })
	}

	panicCases := []struct {
		name string
		src  string
	}{
		{
			"slice shorter than the target array panics",
			`package main
			func Main() int {
				s := []int{1, 2}
				a := [4]int(s)
				return a[0]
			}`,
		},
		{
			"byte slice shorter than the target array panics",
			`package main
			func Main() int {
				s := []byte{1, 2}
				a := [4]byte(s)
				return int(a[0])
			}`,
		},
		{
			"slice shorter than a named target array type panics",
			`package main
			type Arr4 [4]int
			func Main() int {
				s := []int{1, 2}
				a := Arr4(s)
				return a[0]
			}`,
		},
	}
	for _, tc := range panicCases {
		t.Run(tc.name, func(t *testing.T) {
			v := vmAndCompile(t, tc.src)
			require.Error(t, v.Run())
			require.True(t, v.HasFailed())
		})
	}
}

func TestArrayInline(t *testing.T) {
	t.Run("regular array parameter", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
		func Main() int {
			arr := [2]int{0: 42}
			inline.MutateArray(arr)
			return arr[0]
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("variadic array parameter is copied, not aliased", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
		func Main() int {
			arr1 := [2]int{0: 40}
			arr2 := [2]int{0: 2}
			inline.MutateArrays(arr1, arr2)
			return arr1[0] + arr2[0]
		}`
		eval(t, src, big.NewInt(42))
	})
}
