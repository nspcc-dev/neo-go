package vm_test

import (
	"math/big"
	"testing"
)

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
