package compiler_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
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
		"append multiple bytes to a slice",
		`package foo
		func Main() []byte {
			var a []byte
			a = append(a, 1, 2)
			return a
		}`,
		[]byte{1, 2},
	},
	{
		"append multiple ints to a slice",
		`package foo
		func Main() []int {
			var a []int
			a = append(a, 1, 2, 3)
			a = append(a, 4, 5)
			return a
		}`,
		[]stackitem.Item{
			stackitem.NewBigInteger(big.NewInt(1)),
			stackitem.NewBigInteger(big.NewInt(2)),
			stackitem.NewBigInteger(big.NewInt(3)),
			stackitem.NewBigInteger(big.NewInt(4)),
			stackitem.NewBigInteger(big.NewInt(5)),
		},
	},
	{
		"int slice, append slice",
		`package foo
			func getByte() byte { return 0x80 }
			func Main() []int {
				x := []int{1}
				y := []int{2, 3}
				x = append(x, y...)
				x = append(x, y...)
				return x
			}`,
		[]stackitem.Item{
			stackitem.Make(1),
			stackitem.Make(2), stackitem.Make(3),
			stackitem.Make(2), stackitem.Make(3),
		},
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
		[]stackitem.Item{
			stackitem.NewByteArray([]byte("a")),
			stackitem.NewByteArray([]byte("b")),
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
		[]stackitem.Item{
			stackitem.NewByteArray([]byte("a")),
			stackitem.NewByteArray([]byte("b")),
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
	{
		"declare and append byte-slice",
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
		"nested slice assignment",
		`package foo
		func Main() int {
			a := [][]int{[]int{1, 2}, []int{3, 4}}
			a[1][0] = 42
			return a[1][0]
		}`,
		big.NewInt(42),
	},
	{
		"nested slice omitted type (slice)",
		`package foo
		func Main() int {
			a := [][]int{{1, 2}, {3, 4}}
			a[1][0] = 42
			return a[1][0]
		}`,
		big.NewInt(42),
	},
	{
		"nested slice omitted type (struct)",
		`package foo
		type pair struct { a, b int }
		func Main() int {
			a := []pair{{a: 1, b: 2}, {a: 3, b: 4}}
			a[1].a = 42
			return a[1].a
		}`,
		big.NewInt(42),
	},
	{
		"defaults to nil for byte slice",
		`
		package foo
		func Main() int {
			var a []byte
			if a != nil { return 1}
			return 2
		}
		`,
		big.NewInt(2),
	},
	{
		"defaults to nil for int slice",
		`
		package foo
		func Main() int {
			var a []int
			if a != nil { return 1}
			return 2
		}
		`,
		big.NewInt(2),
	},
	{
		"defaults to nil for struct slice",
		`
		package foo
		type pair struct { a, b int }
		func Main() int {
			var a []pair
			if a != nil { return 1}
			return 2
		}
		`,
		big.NewInt(2),
	},
	{
		"literal byte-slice with variable values",
		`package foo
		const sym1 = 's'
		func Main() []byte {
			sym2 := byte('t')
			sym4 := byte('i')
			return []byte{sym1, sym2, 'r', sym4, 'n', 'g'}
		}`,
		[]byte("string"),
	},
	{
		"literal slice with function call",
		`package foo
		func fn() byte { return 't' }
		func Main() []byte {
			return []byte{'s', fn(), 'r'}
		}`,
		[]byte("str"),
	},
}

func TestSliceOperations(t *testing.T) {
	runTestCases(t, sliceTestCases)
}

func TestByteSlices(t *testing.T) {
	testCases := []testCase{
		{
			"append bytes bigger than 0x79",
			`package foo
			func Main() []byte {
				var z []byte
				z = append(z, 0x78, 0x79, 0x80, 0x81, 0xFF)
				return z
			}`,
			[]byte{0x78, 0x79, 0x80, 0x81, 0xFF},
		},
		{
			"append bytes bigger than 0x79, not nil",
			`package foo
			func Main() []byte {
				z := []byte{0x78}
				z = append(z, 0x79, 0x80, 0x81, 0xFF)
				return z
			}`,
			[]byte{0x78, 0x79, 0x80, 0x81, 0xFF},
		},
		{
			"append bytes bigger than 0x79, function return",
			`package foo
			func getByte() byte { return 0x80 }
			func Main() []byte {
				var z []byte
				z = append(z, 0x78, 0x79, getByte(), 0x81, 0xFF)
				return z
			}`,
			[]byte{0x78, 0x79, 0x80, 0x81, 0xFF},
		},
		{
			"append ints bigger than 0x79, function return byte",
			`package foo
			func getByte() byte { return 0x80 }
			func Main() []int {
				var z []int
				z = append(z, 0x78, int(getByte()))
				return z
			}`,
			[]stackitem.Item{stackitem.Make(0x78), stackitem.Make(0x80)},
		},
		{
			"slice literal, bytes bigger than 0x79, function return",
			`package foo
			func getByte() byte { return 0x80 }
			func Main() []byte {
				z := []byte{0x79, getByte(), 0x81}
				return z
			}`,
			[]byte{0x79, 0x80, 0x81},
		},
		{
			"compare bytes as integers",
			`package foo
				func getByte1() byte { return 0x79 }
				func getByte2() byte { return 0x80 }
				func Main() bool {
					return getByte1() < getByte2()
				}`,
			true,
		},
	}
	runTestCases(t, testCases)
}

func TestSubsliceCompound(t *testing.T) {
	src := `package foo
	func Main() []int {
		a := []int{0, 1, 2, 3}
		b := a[1:3]
		return b
	}`
	_, err := compiler.Compile("", strings.NewReader(src))
	require.Error(t, err)
}

func TestRemove(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/util"
		func Main() int {
			a := []int{11, 22, 33}
			util.Remove(a, 1)
			return len(a) + a[0] + a[1]
		}`
		eval(t, src, big.NewInt(46))
	})
	t.Run("ByteSlice", func(t *testing.T) {
		// This test checks that `Remove` has correct arguments.
		// After `Remove` became an opcode it is harder to do such checks.
		// Skip the test for now.
		t.Skip()
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/util"
		func Main() int {
			a := []byte{11, 22, 33}
			util.Remove(a, 1)
			return len(a)
		}`
		_, err := compiler.Compile("", strings.NewReader(src))
		require.Error(t, err)
	})
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

func TestMake(t *testing.T) {
	t.Run("Map", func(t *testing.T) {
		src := `package foo
		func Main() int {
			a := make(map[int]int)
			a[1] = 10
			a[2] = 20
			return a[1]
		}`
		eval(t, src, big.NewInt(10))
	})
	t.Run("IntSlice", func(t *testing.T) {
		src := `package foo
		func Main() int {
			a := make([]int, 10)
			return len(a) + a[0]
		}`
		eval(t, src, big.NewInt(10))
	})
	t.Run("ByteSlice", func(t *testing.T) {
		src := `package foo
		func Main() int {
			a := make([]byte, 10)
			return len(a) + int(a[0])
		}`
		eval(t, src, big.NewInt(10))
	})
	t.Run("CapacityError", func(t *testing.T) {
		src := `package foo
		func Main() int {
			a := make([]int, 1, 2)
			return a[0]
		}`
		_, err := compiler.Compile("foo.go", strings.NewReader(src))
		require.Error(t, err)
	})
}

func TestCopy(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		src := `package foo
		func Main() []int {
			src := []int{3, 2, 1}
			dst := make([]int, 2)
			copy(dst, src)
			return dst
		}`
		_, err := compiler.Compile("foo.go", strings.NewReader(src))
		require.Error(t, err)
	})
	t.Run("Simple", func(t *testing.T) {
		src := `package foo
		func Main() []byte {
			src := []byte{3, 2, 1}
			dst := make([]byte, 2)
			copy(dst, src)
			return dst
		}`
		eval(t, src, []byte{3, 2})
	})
	t.Run("LowSrcIndex", func(t *testing.T) {
		src := `package foo
		func Main() []byte {
			src := []byte{3, 2, 1}
			dst := make([]byte, 2)
			copy(dst, src[1:])
			return dst
		}`
		eval(t, src, []byte{2, 1})
	})
	t.Run("LowDstIndex", func(t *testing.T) {
		src := `package foo
		func Main() []byte {
			src := []byte{3, 2, 1}
			dst := make([]byte, 2)
			copy(dst[1:], src[1:])
			return dst
		}`
		eval(t, src, []byte{0, 2})
	})
	t.Run("BothIndices", func(t *testing.T) {
		src := `package foo
		func Main() []byte {
			src := []byte{4, 3, 2, 1}
			dst := make([]byte, 4)
			copy(dst[1:], src[1:3])
			return dst
		}`
		eval(t, src, []byte{0, 3, 2, 0})
	})
	t.Run("EmptySliceExpr", func(t *testing.T) {
		src := `package foo
		func Main() []byte {
			src := []byte{3, 2, 1}
			dst := make([]byte, 2)
			copy(dst[1:], src[:])
			return dst
		}`
		eval(t, src, []byte{0, 3})
	})
	t.Run("AssignToVariable", func(t *testing.T) {
		src := `package foo
		func Main() int {
			src := []byte{3, 2, 1}
			dst := make([]byte, 2)
			n := copy(dst, src)
			return n
		}`
		eval(t, src, big.NewInt(2))
	})
}
