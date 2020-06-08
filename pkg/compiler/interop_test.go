package compiler_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func TestRemove(t *testing.T) {
	srcTmpl := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/util"
	func Main() int {
		a := %s
		util.Remove(a, %d)
		return len(a) * a[%d]
	}`
	testRemove := func(item string, key, index, result int64) func(t *testing.T) {
		return func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, item, key, index)
			if result > 0 {
				eval(t, src, big.NewInt(result))
				return
			}
			v := vmAndCompile(t, src)
			require.Error(t, v.Run())
		}
	}
	t.Run("Map", func(t *testing.T) {
		item := "map[int]int{1: 2, 5: 7, 11: 13}"
		t.Run("RemovedKey", testRemove(item, 5, 5, -1))
		t.Run("AnotherKey", testRemove(item, 5, 11, 26))
	})
	t.Run("Slice", func(t *testing.T) {
		item := "[]int{5, 7, 11, 13}"
		t.Run("RemovedKey", testRemove(item, 2, 2, 39))
		t.Run("AnotherKey", testRemove(item, 2, 1, 21))
		t.Run("LastKey", testRemove(item, 2, 3, -1))
	})
	t.Run("Invalid", func(t *testing.T) {
		srcTmpl := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/util"
		func Main() int {
			util.Remove(%s, 2)
			return 1
		}`
		t.Run("BasicType", func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, "1")
			_, err := compiler.Compile(strings.NewReader(src))
			require.Error(t, err)
		})
		t.Run("ByteSlice", func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, "[]byte{1, 2}")
			_, err := compiler.Compile(strings.NewReader(src))
			require.Error(t, err)
		})
	})
}

func TestFromAddress(t *testing.T) {
	as1 := "Aej1fe4mUgou48Zzup5j8sPrE3973cJ5oz"
	addr1, err := address.StringToUint160(as1)
	require.NoError(t, err)

	as2 := "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"
	addr2, err := address.StringToUint160(as2)
	require.NoError(t, err)

	t.Run("append 2 addresses", func(t *testing.T) {
		src := `
		package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/util"
		func Main() []byte {
			addr1 := util.FromAddress("` + as1 + `")
			addr2 := util.FromAddress("` + as2 + `")
			sum := append(addr1, addr2...)
			return sum
		}
		`

		eval(t, src, append(addr1.BytesBE(), addr2.BytesBE()...))
	})

	t.Run("append 2 addresses inline", func(t *testing.T) {
		src := `
		package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/util"
		func Main() []byte {
			addr1 := util.FromAddress("` + as1 + `")
			sum := append(addr1, util.FromAddress("` + as2 + `")...)
			return sum
		}
		`

		eval(t, src, append(addr1.BytesBE(), addr2.BytesBE()...))
	})
}

func TestAppCall(t *testing.T) {
	const srcDynApp = `
	package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/engine"
	func Main(h []byte) []byte {
		x := []byte{1, 2}
		y := []byte{3, 4}
		result := engine.DynAppCall(h, x, y)
		return result.([]byte)
	}
`

	var hasDynamicInvoke bool

	srcInner := `
	package foo
	func Main(a []byte, b []byte) []byte {
		return append(a, b...)
	}
	`

	inner, err := compiler.Compile(strings.NewReader(srcInner))
	require.NoError(t, err)

	dynapp, err := compiler.Compile(strings.NewReader(srcDynApp))
	require.NoError(t, err)

	ih := hash.Hash160(inner)
	dh := hash.Hash160(dynapp)
	getScript := func(u util.Uint160) ([]byte, bool) {
		if u.Equals(ih) {
			return inner, true
		}
		if u.Equals(dh) {
			return dynapp, hasDynamicInvoke
		}
		return nil, false
	}

	dynEntryScript := `
	package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/engine"
	func Main(h []byte) interface{} {
		return engine.AppCall(` + fmt.Sprintf("%#v", dh.BytesBE()) + `, h)
	}
`
	dynentry, err := compiler.Compile(strings.NewReader(dynEntryScript))
	require.NoError(t, err)

	t.Run("valid script", func(t *testing.T) {
		src := getAppCallScript(fmt.Sprintf("%#v", ih.BytesBE()))
		v := vmAndCompile(t, src)
		v.SetScriptGetter(getScript)

		require.NoError(t, v.Run())

		assertResult(t, v, []byte{1, 2, 3, 4})
	})

	t.Run("missing script", func(t *testing.T) {
		h := ih
		h[0] = ^h[0]

		src := getAppCallScript(fmt.Sprintf("%#v", h.BytesBE()))
		v := vmAndCompile(t, src)
		v.SetScriptGetter(getScript)

		require.Error(t, v.Run())
	})

	t.Run("invalid script address", func(t *testing.T) {
		src := getAppCallScript("[]byte{1, 2, 3}")

		_, err := compiler.Compile(strings.NewReader(src))
		require.Error(t, err)
	})

	t.Run("convert from string constant", func(t *testing.T) {
		src := `
		package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/engine"
		const scriptHash = ` + fmt.Sprintf("%#v", string(ih.BytesBE())) + `
		func Main() []byte {
			x := []byte{1, 2}
			y := []byte{3, 4}
			result := engine.AppCall([]byte(scriptHash), x, y)
			return result.([]byte)
		}
		`

		v := vmAndCompile(t, src)
		v.SetScriptGetter(getScript)

		require.NoError(t, v.Run())

		assertResult(t, v, []byte{1, 2, 3, 4})
	})

	t.Run("dynamic", func(t *testing.T) {
		t.Run("valid script", func(t *testing.T) {
			hasDynamicInvoke = true
			v := vm.New()
			v.Load(dynentry)
			v.SetScriptGetter(getScript)
			v.Estack().PushVal(ih.BytesBE())

			require.NoError(t, v.Run())

			assertResult(t, v, []byte{1, 2, 3, 4})
		})
		t.Run("invalid script", func(t *testing.T) {
			hasDynamicInvoke = true
			v := vm.New()
			v.Load(dynentry)
			v.SetScriptGetter(getScript)
			v.Estack().PushVal([]byte{1})

			require.Error(t, v.Run())
		})
		t.Run("no dynamic invoke", func(t *testing.T) {
			hasDynamicInvoke = false
			v := vm.New()
			v.Load(dynentry)
			v.SetScriptGetter(getScript)
			v.Estack().PushVal(ih.BytesBE())

			require.Error(t, v.Run())
		})
	})
}

func getAppCallScript(h string) string {
	return `
	package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/engine"
	func Main() []byte {
		x := []byte{1, 2}
		y := []byte{3, 4}
		result := engine.AppCall(` + h + `, x, y)
		return result.([]byte)
	}
	`
}
