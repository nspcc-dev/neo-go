package compiler_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/stretchr/testify/require"
)

func TestImportFunction(t *testing.T) {
	src := `
		package somethingelse

		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/foo"

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

		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/bar"

	 	func Main() int {
			 b := bar.Bar{
				 X: 4,
			 }
			 return b.Y
	 	}
	 `
	eval(t, src, big.NewInt(0))
}

func TestMultipleDirFileImport(t *testing.T) {
	src := `
		package hello

		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/foobar"

		func Main() bool {
			ok := foobar.OtherBool()
			return ok
		}
	`
	eval(t, src, true)
}

func TestImportNameSameAsOwn(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/foo"
	func get3() int { return 3 }
	func Main() int {
		return get3()
	}
	func unused() int {
		return foo.Bar()
	}`
	eval(t, src, big.NewInt(3))
}

func TestImportCycleDirect(t *testing.T) {
	src := `
		package some
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/importcycle/pkg2"
		func Main() int {
			return pkg2.A
		}
	`
	_, _, err := compiler.CompileWithOptions("some.go", strings.NewReader(src), nil)
	require.Error(t, err)
}

func TestImportCycleIndirect(t *testing.T) {
	src := `
		package some
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/importcycle/pkg1"
		func Main() int {
			return pkg1.A
		}
	`
	_, _, err := compiler.CompileWithOptions("some.go", strings.NewReader(src), nil)
	require.Error(t, err)
}
