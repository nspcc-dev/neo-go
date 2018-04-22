package vm_test

import (
	"math/big"
	"testing"
)

func TestImportFunction(t *testing.T) {
	src := `
		package somethingelse

		import "github.com/CityOfZion/neo-go/pkg/vm/tests/foo"

		func Main() int {
			i := foo.NewBar()
			return i
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestImportStruct(t *testing.T) {
	src := `
	 	package somethingwedontcareabout

		import "github.com/CityOfZion/neo-go/pkg/vm/tests/bar"

	 	func Main() int {
			 b := bar.Bar{
				 X: 4,
			 }
			 return b.Y
	 	}
	 `
	eval(t, src, big.NewInt(0))
}
