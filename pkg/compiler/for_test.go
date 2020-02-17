package compiler_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/compiler"

	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func TestEntryPointWithMethod(t *testing.T) {
	src := `
		package foo

		func Main(op string) int {
			if op == "a" {
				return 1
			}
			return 0
		}
	`
	evalWithArgs(t, src, []byte("a"), nil, big.NewInt(1))
}

func TestEntryPointWithArgs(t *testing.T) {
	src := `
		package foo

		func Main(args []interface{}) int {
			return 2 + args[1].(int)
		}
	`
	args := []vm.StackItem{vm.NewBigIntegerItem(0), vm.NewBigIntegerItem(1)}
	evalWithArgs(t, src, nil, args, big.NewInt(3))
}

func TestEntryPointWithMethodAndArgs(t *testing.T) {
	src := `
		package foo

		func Main(method string, args []interface{}) int {
			if method == "foobar" {
				return 2 + args[1].(int)
			}
			return 0
		}
	`
	args := []vm.StackItem{vm.NewBigIntegerItem(0), vm.NewBigIntegerItem(1)}
	evalWithArgs(t, src, []byte("foobar"), args, big.NewInt(3))
}

func TestArrayFieldInStruct(t *testing.T) {
	src := `
		package foo

		type Bar struct {
			arr []int
		}

		func Main() int {
			b := Bar{
				arr: []int{0, 1, 2},
			}
			x := b.arr[2]
			return x + 2
		}
	`
	eval(t, src, big.NewInt(4))
}

func TestArrayItemGetIndexBinaryExpr(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x + 1]
		}
	`
	eval(t, src, big.NewInt(2))
}

func TestArrayItemGetIndexIdent(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x]
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestArrayItemBinExpr(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			return x[1] + 10
		}
	`
	eval(t, src, big.NewInt(11))
}

func TestArrayItemReturn(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{0, 1, 2}
			return arr[1]
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestArrayItemAssign(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{1, 2, 3}
			y := arr[0]
			return y
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestStringArray(t *testing.T) {
	src := `
		package foo
		func Main() []string {
			x := []string{"foo", "bar", "foobar"}
			return x
		}
	`
	eval(t, src, []vm.StackItem{
		vm.NewByteArrayItem([]byte("foo")),
		vm.NewByteArrayItem([]byte("bar")),
		vm.NewByteArrayItem([]byte("foobar")),
	})
}

func TestIntArray(t *testing.T) {
	src := `
		package foo
		func Main() []int {
			arr := []int{1, 2, 3}
			return arr
		}
	`
	eval(t, src, []vm.StackItem{
		vm.NewBigIntegerItem(1),
		vm.NewBigIntegerItem(2),
		vm.NewBigIntegerItem(3),
	})
}

func TestArrayLen(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{0, 1, 2}
			return len(arr)
		}
	`
	eval(t, src, big.NewInt(3))
}

func TestStringLen(t *testing.T) {
	src := `
		package foo
		func Main() int {
			str := "this is medium sized string"
			return len(str)
		}
	`
	eval(t, src, big.NewInt(27))
}

func TestByteArrayLen(t *testing.T) {
	src := `
		package foo

		func Main() int {
			b := []byte{0x00, 0x01, 0x2}
			return len(b)
		}
	`
	eval(t, src, big.NewInt(3))
}

func TestSimpleString(t *testing.T) {
	src := `
		package foo
		func Main() string {
			x := "NEO"
			return x
		}
	`
	eval(t, src, vm.NewByteArrayItem([]byte("NEO")).Value())
}

func TestBoolAssign(t *testing.T) {
	src := `
		package foo
		func Main() bool {
			x := true
			return x
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestBoolCompare(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if x {
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestBoolCompareVerbose(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if x == true {
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestUnaryExpr(t *testing.T) {
	src := `
		package foo
		func Main() bool {
			x := false
			return !x
		}
	`
	eval(t, src, true)
}

func TestIfUnaryInvertPass(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := false
			if !x { 
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestIfUnaryInvert(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if !x { 
				return 10
			}
			return 0
		}
	`
	eval(t, src, []byte{})
}

func TestAppendByte(t *testing.T) {
	src := `
	package foo
		func Main() []byte {
			arr := []byte{0x00, 0x01, 0x02}
			arr = append(arr, 0x03)
			arr = append(arr, 0x04)
			arr = append(arr, 0x05)
			arr = append(arr, 0x06)
			return arr
		}
	`
	eval(t, src, []uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
}

func TestAppendByteToEmpty(t *testing.T) {
	src := `
	package foo
	func Main() []byte {
		out := []byte{}
		out = append(out, 1)
		out = append(out, 2)
		return out
	}`
	eval(t, src, []byte{1, 2})
}

func TestAppendString(t *testing.T) {
	src := `
	package foo
		func Main() string {
			arr := []string{"a", "b", "c"}
			arr = append(arr, "d")
			return arr[3]
		}
	`
	eval(t, src, vm.NewByteArrayItem([]byte("d")).Value())
}

func TestAppendInt(t *testing.T) {
	src := `
	package foo
		func Main() int {
			arr := []int{0, 1, 2}
			arr = append(arr, 3)
			return arr[3]
		}
	`
	eval(t, src, big.NewInt(3))
}

func TestClassicForLoop(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 0
			for i := 0; i < 10; i++ {
				x = i
			}
			return x
		}
	`
	eval(t, src, big.NewInt(9))
}

func TestInc(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 0
			x++
			return x
		}
	`

	eval(t, src, big.NewInt(1))
}

func TestDec(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 2
			x--
			return x
		}
	`

	eval(t, src, big.NewInt(1))
}

func TestForLoopBigIter(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 0
			for i := 0; i < 100000; i++ {
				x = i
			}
			return x
		}
	`
	eval(t, src, big.NewInt(99999))
}

func TestForLoopNoInit(t *testing.T) {
	src := `
	package foo
		func Main() int {
			i := 0
			for ; i < 10; i++ {
			}
			return i
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestForLoopNoPost(t *testing.T) {
	src := `
	package foo
		func Main() int {
			i := 0
			for i < 10 {
				i++
			}
			return i
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestForLoopRange(t *testing.T) {
	src := `
	package foo
	func Main() int {
		sum := 0
		arr := []int{1, 2, 3}
		for i := range arr {
			sum += arr[i] 
		}
		return sum
	}`

	eval(t, src, big.NewInt(6))
}

func TestForLoopRangeGlobalIndex(t *testing.T) {
	src := `
	package foo
	func Main() int {
		sum := 0
		i := 0
		arr := []int{1, 2, 3}
		for i = range arr {
			sum += arr[i] 
		}
		return sum + i
	}`

	eval(t, src, big.NewInt(8))
}

func TestForLoopRangeChangeVariable(t *testing.T) {
	src := `
	package foo
	func Main() int {
		sum := 0
		arr := []int{1, 2, 3}
		for i := range arr {
			sum += arr[i]
			i++
			sum += i
		}
		return sum
	}`

	eval(t, src, big.NewInt(12))
}

func TestForLoopBreak(t *testing.T) {
	src := `
	package foo
	func Main() int {
		var i int
		for i < 10 {
			i++
			if i == 5 {
				break
			}
		}
		return i
	}`

	eval(t, src, big.NewInt(5))
}

func TestForLoopBreakLabel(t *testing.T) {
	src := `
	package foo
	func Main() int {
		var i int
		loop:
		for i < 10 {
			i++
			if i == 5 {
				break loop
			}
		}
		return i
	}`

	eval(t, src, big.NewInt(5))
}

func TestForLoopNestedBreak(t *testing.T) {
	src := `
	package foo
	func Main() int {
		var i int
		for i < 10 {
			i++
			for j := 0; j < 2; j++ {
				i++
				if i == 5 {
					break
				}
			}
		}
		return i
	}`

	eval(t, src, big.NewInt(11))
}

func TestForLoopNestedBreakLabel(t *testing.T) {
	src := `
	package foo
	func Main() int {
		var i int
		loop:
		for i < 10 {
			i++
			for j := 0; j < 2; j++ {
				if i == 5 {
					break loop
				}
				i++
			}
		}
		return i
	}`

	eval(t, src, big.NewInt(5))
}

func TestForLoopRangeNoVariable(t *testing.T) {
	src := `
	package foo
	func Main() int {
		sum := 0
		arr := []int{1, 2, 3}
		for range arr {
			sum += 1
		}
		return sum
	}`

	eval(t, src, big.NewInt(3))
}

func TestForLoopRangeCompilerError(t *testing.T) {
	src := `
	package foo
	func f(a int) int { return 0 }
	func Main() int {
		arr := []int{1, 2, 3}
		for _, v := range arr {
			f(v)
		}
		return 0
	}`

	_, err := compiler.Compile(strings.NewReader(src))
	require.Error(t, err)
}
