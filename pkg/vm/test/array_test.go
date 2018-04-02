package vm_test

import (
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/vm"
)

var arrayTestCases = []testCase{
	{
		"assign int array",
		`
		package foo
		func Main() []int {
			x := []int{1, 2, 3}
			return x
		}
		`,
		[]vm.StackItem{
			vm.NewBigIntegerItem(1),
			vm.NewBigIntegerItem(2),
			vm.NewBigIntegerItem(3),
		},
	},
	{
		"assign string array",
		`
		package foo
		func Main() []string {
			x := []string{"foo", "bar", "foobar"}
			return x
		}
		`,
		[]vm.StackItem{
			vm.NewByteArrayItem([]byte("foo")),
			vm.NewByteArrayItem([]byte("bar")),
			vm.NewByteArrayItem([]byte("foobar")),
		},
	},
	{
		"array item assign",
		`
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			y := x[0]
			return y
		}
		`,
		big.NewInt(0),
	},
	{
		"array item return",
		`
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			return x[1]
		}
		`,
		big.NewInt(1),
	},
	{
		"array item in bin expr",
		`
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			return x[1] + 10
		}
		`,
		big.NewInt(11),
	},
	{
		"array item ident",
		`
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x]
		}
		`,
		big.NewInt(1),
	},
	{
		"array item index with binExpr",
		`
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x + 1]
		}
		`,
		big.NewInt(2),
	},
	{
		"array item struct",
		`
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
		`,
		big.NewInt(4),
	},
}
