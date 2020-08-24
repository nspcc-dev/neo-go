package compiler_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package foo
		var a int
		func init() {
			a = 42
		}
		func Main() int {
			return a
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("Multi", func(t *testing.T) {
		src := `package foo
		var m = map[int]int{}
		var a = 2
		func init() {
			m[1] = 11
		}
		func init() {
			a = 1
			m[3] = 30
		}
		func Main() int {
			return m[1] + m[3] + a
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("WithCall", func(t *testing.T) {
		src := `package foo
		var m = map[int]int{}
		func init() {
			initMap(m)
		}
		func initMap(m map[int]int) {
			m[11] = 42
		}
		func Main() int {
			return m[11]
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("InvalidSignature", func(t *testing.T) {
		src := `package foo
		type Foo int
		var a int
		func (f Foo) init() {
			a = 2
		}
		func Main() int {
			return a
		}`
		eval(t, src, big.NewInt(0))
	})
}

func TestImportOrder(t *testing.T) {
	t.Run("1,2", func(t *testing.T) {
		src := `package foo
		import _ "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/pkg1"
		import _ "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/pkg2"
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/pkg3"
		func Main() int { return pkg3.A }`
		eval(t, src, big.NewInt(2))
	})
	t.Run("2,1", func(t *testing.T) {
		src := `package foo
		import _ "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/pkg2"
		import _ "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/pkg1"
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/pkg3"
		func Main() int { return pkg3.A }`
		eval(t, src, big.NewInt(1))
	})
	t.Run("InitializeOnce", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/pkg3"
		var A = pkg3.A
		func Main() int { return A }`
		eval(t, src, big.NewInt(3))
	})
}

func TestInitWithNoGlobals(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	func init() {
		runtime.Notify("called in '_initialize'")
	}
	func Main() int {
		return 42
	}`
	v, s := vmAndCompileInterop(t, src)
	require.NoError(t, v.Run())
	assertResult(t, v, big.NewInt(42))
	require.True(t, len(s.events) == 1)
}
