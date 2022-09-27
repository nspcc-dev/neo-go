package inline

import (
	"github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline/a"
	"github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline/b"
)

func NoArgsNoReturn() {}
func NoArgsReturn1() int {
	return 1
}
func Sum(a, b int) int {
	return a + b
}
func sum(x, y int) int {
	return x + y
}
func SumSquared(a, b int) int {
	return sum(a, b) * (a + b)
}

var A = 1

func GetSumSameName() int {
	return a.GetA() + b.GetA() + A
}

func DropInsideInline() int {
	sum(1, 2)
	sum(3, 4)
	return 7
}

func VarSum(a int, b ...int) int {
	sum := a
	for i := range b {
		sum += b[i]
	}
	return sum
}

func SumVar(a, b int) int {
	return VarSum(a, b)
}

func Concat(n int) int {
	return n*100 + b.A*10 + A
}

type T struct {
	N int
}

func (t *T) Inc(i int) int {
	n := t.N
	t.N += i
	return n
}

func NewT() T {
	return T{N: 42}
}

func (t T) GetN() int {
	return t.N
}

func AppendInsideInline(val []byte) []byte {
	inlinedType := []byte{1, 2, 3}
	return append(inlinedType, val...)
}
