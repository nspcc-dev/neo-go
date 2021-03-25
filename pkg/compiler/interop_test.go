package compiler_test

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	cinterop "github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestTypeConstantSize(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop"
	var a %T // type declaration is always ok
	func Main() interface{} {
		return %#v
	}`

	t.Run("Hash160", func(t *testing.T) {
		t.Run("good", func(t *testing.T) {
			a := make(cinterop.Hash160, 20)
			src := fmt.Sprintf(src, a, a)
			eval(t, src, []byte(a))
		})
		t.Run("bad", func(t *testing.T) {
			a := make(cinterop.Hash160, 19)
			src := fmt.Sprintf(src, a, a)
			_, err := compiler.Compile("foo.go", strings.NewReader(src))
			require.Error(t, err)
		})
	})
	t.Run("Hash256", func(t *testing.T) {
		t.Run("good", func(t *testing.T) {
			a := make(cinterop.Hash256, 32)
			src := fmt.Sprintf(src, a, a)
			eval(t, src, []byte(a))
		})
		t.Run("bad", func(t *testing.T) {
			a := make(cinterop.Hash256, 31)
			src := fmt.Sprintf(src, a, a)
			_, err := compiler.Compile("foo.go", strings.NewReader(src))
			require.Error(t, err)
		})
	})
}

func TestFromAddress(t *testing.T) {
	as1 := "NQRLhCpAru9BjGsMwk67vdMwmzKMRgsnnN"
	addr1, err := address.StringToUint160(as1)
	require.NoError(t, err)

	as2 := "NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8"
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

	t.Run("AliasPackage", func(t *testing.T) {
		src := `
		package foo
		import uu "github.com/nspcc-dev/neo-go/pkg/interop/util"
		func Main() []byte {
			addr1 := uu.FromAddress("` + as1 + `")
			addr2 := uu.FromAddress("` + as2 + `")
			sum := append(addr1, addr2...)
			return sum
		}`
		eval(t, src, append(addr1.BytesBE(), addr2.BytesBE()...))
	})
}

func spawnVM(t *testing.T, ic *interop.Context, src string) *vm.VM {
	b, di, err := compiler.CompileWithDebugInfo("foo.go", strings.NewReader(src))
	require.NoError(t, err)
	v := core.SpawnVM(ic)
	invokeMethod(t, testMainIdent, b, v, di)
	v.LoadScriptWithFlags(b, callflag.All)
	return v
}

func TestAppCall(t *testing.T) {
	srcDeep := `package foo
	func Get42() int {
		return 42
	}`
	barCtr, di, err := compiler.CompileWithDebugInfo("bar.go", strings.NewReader(srcDeep))
	require.NoError(t, err)
	mBar, err := di.ConvertToManifest(&compiler.Options{Name: "Bar"})
	require.NoError(t, err)

	barH := hash.Hash160(barCtr)

	srcInner := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
	import "github.com/nspcc-dev/neo-go/pkg/interop"
	var a int = 3
	func Main(a []byte, b []byte) []byte {
		panic("Main was called")
	}
	func Append(a []byte, b []byte) []byte {
		return append(a, b...)
	}
	func Add3(n int) int {
		return a + n
	}
	func CallInner() int {
		return contract.Call(%s, "get42", contract.All).(int)
	}`
	srcInner = fmt.Sprintf(srcInner,
		fmt.Sprintf("%#v", cinterop.Hash160(barH.BytesBE())))

	inner, di, err := compiler.CompileWithDebugInfo("foo.go", strings.NewReader(srcInner))
	require.NoError(t, err)
	m, err := di.ConvertToManifest(&compiler.Options{Name: "Foo"})
	require.NoError(t, err)

	ih := hash.Hash160(inner)
	var contractGetter = func(_ dao.DAO, h util.Uint160) (*state.Contract, error) {
		if h.Equals(ih) {
			innerNef, err := nef.NewFile(inner)
			require.NoError(t, err)
			return &state.Contract{
				ContractBase: state.ContractBase{
					Hash:     ih,
					NEF:      *innerNef,
					Manifest: *m,
				},
			}, nil
		} else if h.Equals(barH) {
			barNef, err := nef.NewFile(barCtr)
			require.NoError(t, err)
			return &state.Contract{
				ContractBase: state.ContractBase{
					Hash:     barH,
					NEF:      *barNef,
					Manifest: *mBar,
				},
			}, nil
		}
		return nil, errors.New("not found")
	}

	fc := fakechain.NewFakeChain()
	ic := interop.NewContext(trigger.Application, fc, dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false), contractGetter, nil, nil, nil, zaptest.NewLogger(t))

	t.Run("valid script", func(t *testing.T) {
		src := getAppCallScript(fmt.Sprintf("%#v", ih.BytesBE()))
		v := spawnVM(t, ic, src)
		require.NoError(t, v.Run())

		assertResult(t, v, []byte{1, 2, 3, 4})
	})

	t.Run("callEx, valid", func(t *testing.T) {
		src := getCallExScript(fmt.Sprintf("%#v", ih.BytesBE()), "contract.ReadStates|contract.AllowCall")
		v := spawnVM(t, ic, src)
		require.NoError(t, v.Run())

		assertResult(t, v, big.NewInt(42))
	})
	t.Run("callEx, missing flags", func(t *testing.T) {
		src := getCallExScript(fmt.Sprintf("%#v", ih.BytesBE()), "contract.NoneFlag")
		v := spawnVM(t, ic, src)
		require.Error(t, v.Run())
	})

	t.Run("missing script", func(t *testing.T) {
		h := ih
		h[0] = ^h[0]

		src := getAppCallScript(fmt.Sprintf("%#v", h.BytesBE()))
		v := spawnVM(t, ic, src)
		require.Error(t, v.Run())
	})

	t.Run("convert from string constant", func(t *testing.T) {
		src := `
		package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
		const scriptHash = ` + fmt.Sprintf("%#v", string(ih.BytesBE())) + `
		func Main() []byte {
			x := []byte{1, 2}
			y := []byte{3, 4}
			result := contract.Call([]byte(scriptHash), "append", contract.All, x, y)
			return result.([]byte)
		}
		`

		v := spawnVM(t, ic, src)
		require.NoError(t, v.Run())

		assertResult(t, v, []byte{1, 2, 3, 4})
	})

	t.Run("convert from var", func(t *testing.T) {
		src := `
		package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
		func Main() []byte {
			x := []byte{1, 2}
			y := []byte{3, 4}
			var addr = []byte(` + fmt.Sprintf("%#v", string(ih.BytesBE())) + `)
			result := contract.Call(addr, "append", contract.All, x, y)
			return result.([]byte)
		}
		`

		v := spawnVM(t, ic, src)
		require.NoError(t, v.Run())

		assertResult(t, v, []byte{1, 2, 3, 4})
	})

	t.Run("InitializedGlobals", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
		func Main() int {
			var addr = []byte(` + fmt.Sprintf("%#v", string(ih.BytesBE())) + `)
			result := contract.Call(addr, "add3", contract.All, 39)
			return result.(int)
		}`

		v := spawnVM(t, ic, src)
		require.NoError(t, v.Run())

		assertResult(t, v, big.NewInt(42))
	})

	t.Run("AliasPackage", func(t *testing.T) {
		src := `package foo
		import ee "github.com/nspcc-dev/neo-go/pkg/interop/contract"
		func Main() int {
			var addr = []byte(` + fmt.Sprintf("%#v", string(ih.BytesBE())) + `)
			result := ee.Call(addr, "add3", ee.All, 39)
			return result.(int)
		}`
		v := spawnVM(t, ic, src)
		require.NoError(t, v.Run())
		assertResult(t, v, big.NewInt(42))
	})
}

func getAppCallScript(h string) string {
	return `
	package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
	func Main() []byte {
		x := []byte{1, 2}
		y := []byte{3, 4}
		result := contract.Call(` + h + `, "append", contract.All, x, y)
		return result.([]byte)
	}
	`
}

func getCallExScript(h string, flags string) string {
	return `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
	func Main() int {
		result := contract.Call(` + h + `, "callInner", ` + flags + `)
		return result.(int)
	}`
}

func TestBuiltinDoesNotCompile(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/util"
	func Main() bool {
		a := 1
		b := 2
		return util.Equals(a, b)
	}`

	v := vmAndCompile(t, src)
	ctx := v.Context()
	retCount := 0
	for op, _, err := ctx.Next(); err == nil; op, _, err = ctx.Next() {
		if ctx.IP() >= len(ctx.Program()) {
			break
		}
		if op == opcode.RET {
			retCount++
		}
	}
	require.Equal(t, 1, retCount)
}

func TestInteropPackage(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/block"
	func Main() int {
		b := block.Block{}
		a := block.GetTransactionCount(b)
		return a
	}`
	eval(t, src, big.NewInt(42))
}

func TestBuiltinPackage(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/util"
	func Main() int {
		if util.Equals(1, 2) { // always returns true
			return 1
		}
		return 2
	}`
	eval(t, src, big.NewInt(1))
}

func TestLenForNil(t *testing.T) {
	src := `
	package foo
	func Main() bool {
		var a []int = nil
		return len(a) == 0
	}`

	eval(t, src, true)
}
