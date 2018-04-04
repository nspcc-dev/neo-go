package vm_test

import (
	"math/big"
	"testing"
)

func TestAppendString(t *testing.T) {
	src := `
	package foo
		func Main() string {
			arr := []string{"a", "b", "c"}
			arr = append(arr, "d")
			return arr[3]
		}
	`
	eval(t, src, []uint8{byte('d')})
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

// TODO: This could be a nasty bug. Output of the VM is 65695.
// Only happens above 100000, could be binary read issue.
//func TestForLoopBigIter(t *testing.T) {
//	src := `
//	package foo
//		func Main() int {
//			x := 0
//			for i := 0; i < 100000; i++ {
//				x = i
//			}
//			return x
//		}
//	`
//	eval(t, src, big.NewInt(99999))
//}
