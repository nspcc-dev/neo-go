package compiler_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

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

func spawnVM(t *testing.T, ic *interop.Context, src string) *vm.VM {
	b, err := compiler.Compile(strings.NewReader(src))
	require.NoError(t, err)
	v := core.SpawnVM(ic)
	v.Load(b)
	return v
}

func TestAppCall(t *testing.T) {
	srcInner := `
	package foo
	func Main(a []byte, b []byte) []byte {
		return append(a, b...)
	}
	`

	inner, err := compiler.Compile(strings.NewReader(srcInner))
	require.NoError(t, err)

	ic := interop.NewContext(trigger.Application, nil, dao.NewSimple(storage.NewMemoryStore()), nil, nil, nil, zaptest.NewLogger(t))
	require.NoError(t, ic.DAO.PutContractState(&state.Contract{Script: inner}))

	ih := hash.Hash160(inner)
	t.Run("valid script", func(t *testing.T) {
		src := getAppCallScript(fmt.Sprintf("%#v", ih.BytesBE()))
		v := spawnVM(t, ic, src)
		require.NoError(t, v.Run())

		assertResult(t, v, []byte{1, 2, 3, 4})
	})

	t.Run("missing script", func(t *testing.T) {
		h := ih
		h[0] = ^h[0]

		src := getAppCallScript(fmt.Sprintf("%#v", h.BytesBE()))
		v := spawnVM(t, ic, src)
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

		v := spawnVM(t, ic, src)
		require.NoError(t, v.Run())

		assertResult(t, v, []byte{1, 2, 3, 4})
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
