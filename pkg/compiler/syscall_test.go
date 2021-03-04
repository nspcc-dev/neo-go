package compiler_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Checks that changes in `smartcontract` are reflected in compiler interop package.
func TestCallFlags(t *testing.T) {
	require.EqualValues(t, contract.ReadStates, callflag.ReadStates)
	require.EqualValues(t, contract.WriteStates, callflag.WriteStates)
	require.EqualValues(t, contract.AllowCall, callflag.AllowCall)
	require.EqualValues(t, contract.AllowNotify, callflag.AllowNotify)
	require.EqualValues(t, contract.States, callflag.States)
	require.EqualValues(t, contract.ReadOnly, callflag.ReadOnly)
	require.EqualValues(t, contract.All, callflag.All)
	require.EqualValues(t, contract.NoneFlag, callflag.NoneFlag)
}

func TestFindFlags(t *testing.T) {
	require.EqualValues(t, storage.None, istorage.FindDefault)
	require.EqualValues(t, storage.KeysOnly, istorage.FindKeysOnly)
	require.EqualValues(t, storage.RemovePrefix, istorage.FindRemovePrefix)
	require.EqualValues(t, storage.ValuesOnly, istorage.FindValuesOnly)
	require.EqualValues(t, storage.DeserializeValues, istorage.FindDeserialize)
	require.EqualValues(t, storage.PickField0, istorage.FindPick0)
	require.EqualValues(t, storage.PickField1, istorage.FindPick1)
}

func TestStoragePutGet(t *testing.T) {
	src := `
		package foo

		import "github.com/nspcc-dev/neo-go/pkg/interop/storage"

		func Main() string {
			ctx := storage.GetContext()
			key := []byte("token")
			storage.Put(ctx, key, []byte("foo"))
			x := storage.Get(ctx, key)
			return x.(string)
		}
	`
	eval(t, src, []byte("foo"))
}

func TestNotify(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	func Main(arg int) {
		runtime.Notify("Event1", arg, "sum", arg+1)
		runtime.Notify("single")
	}`

	v, s := vmAndCompileInterop(t, src)
	v.Estack().PushVal(11)

	require.NoError(t, v.Run())
	require.Equal(t, 2, len(s.events))

	exp0 := []stackitem.Item{stackitem.NewBigInteger(big.NewInt(11)), stackitem.NewByteArray([]byte("sum")), stackitem.NewBigInteger(big.NewInt(12))}
	assert.Equal(t, "Event1", s.events[0].Name)
	assert.Equal(t, exp0, s.events[0].Item.Value())
	assert.Equal(t, "single", s.events[1].Name)
	assert.Equal(t, []stackitem.Item{}, s.events[1].Item.Value())
}

func TestSyscallInGlobalInit(t *testing.T) {
	src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/binary"
		var a = binary.Base58Decode([]byte("5T"))
		func Main() []byte {
			return a
		}`
	v, s := vmAndCompileInterop(t, src)
	s.interops[interopnames.ToID([]byte(interopnames.SystemBinaryBase58Decode))] = func(v *vm.VM) error {
		s := v.Estack().Pop().Value().([]byte)
		require.Equal(t, "5T", string(s))
		v.Estack().PushVal([]byte{1, 2})
		return nil
	}
	require.NoError(t, v.Run())
	require.Equal(t, []byte{1, 2}, v.Estack().Pop().Value())
}

func TestOpcode(t *testing.T) {
	t.Run("1 argument", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
		func abs(a int) int {
			return neogointernal.Opcode1("ABS", a).(int)
		}
		func Main() int {
			return abs(-42)
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("2 arguments", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
		func add3(a, b, c int) int {
			return neogointernal.Opcode2("SUB", a,
				neogointernal.Opcode2("SUB", b, c).(int)).(int)
		}
		func Main() int {
			return add3(53, 12, 1)
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("POW", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() int {
			return math.Pow(2, math.Pow(3, 2))
		}`
		eval(t, src, big.NewInt(512))
	})
	t.Run("SRQT", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() int {
			return math.Sqrt(math.Sqrt(101)) // == sqrt(10) == 3
		}`
		eval(t, src, big.NewInt(3))
	})
	t.Run("SIGN", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() []int {
			signs := make([]int, 3)
			signs[0] = math.Sign(-123)
			signs[1] = math.Sign(0)
			signs[2] = math.Sign(42)
			return signs
		}`
		eval(t, src, []stackitem.Item{
			stackitem.Make(-1),
			stackitem.Make(0),
			stackitem.Make(1),
		})
	})
	t.Run("ABS", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() int {
			return math.Abs(-3)
		}`
		eval(t, src, big.NewInt(3))
	})
	t.Run("MAX", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() int {
			return math.Max(1, 2) + math.Max(8, 3)
		}`
		eval(t, src, big.NewInt(10))
	})
	t.Run("MIN", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() int {
			return math.Min(1, 2) + math.Min(8, 3)
		}`
		eval(t, src, big.NewInt(4))
	})
	t.Run("WITHIN", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() []bool {
			r := make([]bool, 5)
			r[0] = math.Within(2, 3, 5)
			r[1] = math.Within(3, 3, 5)
			r[2] = math.Within(4, 3, 5)
			r[3] = math.Within(5, 3, 5)
			r[4] = math.Within(6, 3, 5)
			return r
		}`
		eval(t, src, []stackitem.Item{
			stackitem.Make(false),
			stackitem.Make(true),
			stackitem.Make(true),
			stackitem.Make(false),
			stackitem.Make(false),
		})
	})
}
