package compiler_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm"
)

var sliceTestCases = []testCase{
	{
		"constant index",
		`
		package foo
		func Main() int {
			a := []int{0,0}
			a[1] = 42
			return a[1]+0
		}
		`,
		big.NewInt(42),
	},
	{
		"variable index",
		`
		package foo
		func Main() int {
			a := []int{0,0}
			i := 1
			a[i] = 42
			return a[1]+0
		}
		`,
		big.NewInt(42),
	},
	{
		"increase slice element with +=",
		`package foo
		func Main() int {
			a := []int{1, 2, 3}
			a[1] += 40
			return a[1]
		}`,
		big.NewInt(42),
	},
	{
		"complex test",
		`
		package foo
		func Main() int {
			a := []int{1,2,3}
			x := a[0]
			a[x] = a[x] + 4
			a[x] = a[x] + a[2]
			return a[1]
		}
		`,
		big.NewInt(9),
	},
	{
		"slice literals with variables",
		`
		package foo
		func Main() int {
			elem := 7
			a := []int{6, elem, 8}
			return a[1]
		}
		`,
		big.NewInt(7),
	},
	{
		"slice literals with expressions",
		`
		package foo
		func Main() int {
			elem := []int{3, 7}
			a := []int{6, elem[1]*2+1, 24}
			return a[1]
		}
		`,
		big.NewInt(15),
	},
	{
		"sub-slice with literal bounds",
		`
		package foo
		func Main() []byte {
			a := []byte{0, 1, 2, 3}
			b := a[1:3]
			return b
		}`,
		[]byte{1, 2},
	},
	{
		"sub-slice with constant bounds",
		`
		package foo
		const x = 1
		const y = 3
		func Main() []byte {
			a := []byte{0, 1, 2, 3}
			b := a[x:y]
			return b
		}`,
		[]byte{1, 2},
	},
	{
		"sub-slice with variable bounds",
		`
		package foo
		func Main() []byte {
			a := []byte{0, 1, 2, 3}
			x := 1
			y := 3
			b := a[x:y]
			return b
		}`,
		[]byte{1, 2},
	},
	{
		"sub-slice with no lower bound",
		`
		package foo
		func Main() []byte {
			a := []byte{0, 1, 2, 3}
			b := a[:3]
			return b
		}`,
		[]byte{0, 1, 2},
	},
	{
		"sub-slice with no upper bound",
		`
		package foo
		func Main() []byte {
			a := []byte{0, 1, 2, 3}
			b := a[2:]
			return b
		}`,
		[]byte{2, 3},
	},
	{
		"declare byte slice",
		`package foo
		func Main() []byte {
			var a []byte
			a = append(a, 1)
			a = append(a, 2)
			return a
		}`,
		[]byte{1, 2},
	},
	{
		"declare compound slice",
		`package foo
		func Main() []string {
			var a []string
			a = append(a, "a")
			a = append(a, "b")
			return a
		}`,
		[]vm.StackItem{
			vm.NewByteArrayItem([]byte("a")),
			vm.NewByteArrayItem([]byte("b")),
		},
	},
	{
		"declare compound slice alias",
		`package foo
		type strs []string
		func Main() []string {
			var a strs
			a = append(a, "a")
			a = append(a, "b")
			return a
		}`,
		[]vm.StackItem{
			vm.NewByteArrayItem([]byte("a")),
			vm.NewByteArrayItem([]byte("b")),
		},
	},
	{
		"byte-slice assignment",
		`package foo
		func Main() []byte {
			a := []byte{0, 1, 2}
			a[1] = 42
			return a
		}`,
		[]byte{0, 42, 2},
	},
	{
		"byte-slice assignment after string conversion",
		`package foo
		func Main() []byte {
			a := "abc"
			b := []byte(a)
			b[1] = 42
			return []byte(a)
		}`,
		[]byte{0x61, 0x62, 0x63},
	},
}

func TestSliceOperations(t *testing.T) {
	runTestCases(t, sliceTestCases)
}

func TestJumps(t *testing.T) {
	src := `
	package foo
	func Main() []byte {
		buf := []byte{0x62, 0x01, 0x00}
		return buf
	}
	`
	eval(t, src, []byte{0x62, 0x01, 0x00})
}
