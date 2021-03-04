package compiler_test

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
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

type syscallTestCase struct {
	method string
	params []string
	isVoid bool
}

// This test ensures that our wrappers have necessary number of parameters
// and execute needed syscall. Because of lack of typing (compared to native contracts)
// parameter types can't be checked.
func TestSyscallExecution(t *testing.T) {
	b := `[]byte{1}`
	u160 := `interop.Hash160("aaaaaaaaaaaaaaaaaaaa")`
	pub := `interop.PublicKey("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")`
	pubs := "[]interop.PublicKey{ " + pub + "}"
	sig := `interop.Signature("aaaaaa")`
	sigs := "[]interop.Signature{" + sig + "}"
	sctx := "storage.Context{}"
	interops := map[string]syscallTestCase{
		"binary.Deserialize":                 {interopnames.SystemBinaryDeserialize, []string{b}, false},
		"binary.Serialize":                   {interopnames.SystemBinarySerialize, []string{"10"}, false},
		"contract.Call":                      {interopnames.SystemContractCall, []string{u160, `"m"`, "1", "3"}, false},
		"contract.CreateMultisigAccount":     {interopnames.SystemContractCreateMultisigAccount, []string{"1", pubs}, false},
		"contract.CreateStandardAccount":     {interopnames.SystemContractCreateStandardAccount, []string{pub}, false},
		"contract.IsStandard":                {interopnames.SystemContractIsStandard, []string{u160}, false},
		"contract.GetCallFlags":              {interopnames.SystemContractGetCallFlags, nil, false},
		"iterator.Create":                    {interopnames.SystemIteratorCreate, []string{pubs}, false},
		"iterator.Next":                      {interopnames.SystemIteratorNext, []string{"iterator.Iterator{}"}, false},
		"iterator.Value":                     {interopnames.SystemIteratorValue, []string{"iterator.Iterator{}"}, false},
		"runtime.CheckWitness":               {interopnames.SystemRuntimeCheckWitness, []string{b}, false},
		"runtime.GasLeft":                    {interopnames.SystemRuntimeGasLeft, nil, false},
		"runtime.GetCallingScriptHash":       {interopnames.SystemRuntimeGetCallingScriptHash, nil, false},
		"runtime.GetEntryScriptHash":         {interopnames.SystemRuntimeGetEntryScriptHash, nil, false},
		"runtime.GetExecutingScriptHash":     {interopnames.SystemRuntimeGetExecutingScriptHash, nil, false},
		"runtime.GetInvocationCounter":       {interopnames.SystemRuntimeGetInvocationCounter, nil, false},
		"runtime.GetNotifications":           {interopnames.SystemRuntimeGetNotifications, []string{u160}, false},
		"runtime.GetScriptContainer":         {interopnames.SystemRuntimeGetScriptContainer, nil, false},
		"runtime.GetTime":                    {interopnames.SystemRuntimeGetTime, nil, false},
		"runtime.GetTrigger":                 {interopnames.SystemRuntimeGetTrigger, nil, false},
		"runtime.Log":                        {interopnames.SystemRuntimeLog, []string{`"msg"`}, true},
		"runtime.Notify":                     {interopnames.SystemRuntimeNotify, []string{`"ev"`, "1"}, true},
		"runtime.Platform":                   {interopnames.SystemRuntimePlatform, nil, false},
		"storage.Delete":                     {interopnames.SystemStorageDelete, []string{sctx, b}, true},
		"storage.Find":                       {interopnames.SystemStorageFind, []string{sctx, b, "storage.None"}, false},
		"storage.Get":                        {interopnames.SystemStorageGet, []string{sctx, b}, false},
		"storage.GetContext":                 {interopnames.SystemStorageGetContext, nil, false},
		"storage.GetReadOnlyContext":         {interopnames.SystemStorageGetReadOnlyContext, nil, false},
		"storage.Put":                        {interopnames.SystemStoragePut, []string{sctx, b, b}, true},
		"storage.ConvertContextToReadOnly":   {interopnames.SystemStorageAsReadOnly, []string{sctx}, false},
		"crypto.ECDsaSecp256r1Verify":        {interopnames.NeoCryptoVerifyWithECDsaSecp256r1, []string{b, pub, sig}, false},
		"crypto.ECDsaSecp256k1Verify":        {interopnames.NeoCryptoVerifyWithECDsaSecp256k1, []string{b, pub, sig}, false},
		"crypto.ECDSASecp256r1CheckMultisig": {interopnames.NeoCryptoCheckMultisigWithECDsaSecp256r1, []string{b, pubs, sigs}, false},
		"crypto.ECDSASecp256k1CheckMultisig": {interopnames.NeoCryptoCheckMultisigWithECDsaSecp256k1, []string{b, pubs, sigs}, false},
	}
	ic := &interop.Context{}
	core.SpawnVM(ic) // set Functions field
	for _, fs := range ic.Functions {
		for i := range fs {
			// It will be set in test and we want to fail if calling invalid syscall.
			fs[i].Func = nil
		}
	}
	for goName, tc := range interops {
		t.Run(goName, func(t *testing.T) {
			runSyscallTestCase(t, ic, goName, tc)
		})
	}
}

func runSyscallTestCase(t *testing.T, ic *interop.Context, goName string, tc syscallTestCase) {
	syscallID := interopnames.ToID([]byte(tc.method))
	f := ic.GetFunction(syscallID)
	require.NotNil(t, f)
	require.Equal(t, f.ParamCount, len(tc.params))
	called := false
	f.Func = func(ic *interop.Context) error {
		called = true
		if ic.VM.Estack().Len() < f.ParamCount {
			return errors.New("not enough parameters")
		}
		for i := 0; i < f.ParamCount; i++ {
			ic.VM.Estack().Pop()
		}
		if !tc.isVoid {
			ic.VM.Estack().PushVal(42)
		}
		return nil
	}
	defer func() { f.Func = nil }()

	srcTmpl := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/%s"
	import "github.com/nspcc-dev/neo-go/pkg/interop"
	func unused() { var _ interop.Hash160 }
	`
	if tc.isVoid {
		srcTmpl += `func Main() { %s(%s) }`
	} else {
		srcTmpl += `func Main() interface{} { return %s(%s) }`
	}
	ss := strings.Split(goName, ".")
	src := fmt.Sprintf(srcTmpl, ss[0], goName, strings.Join(tc.params, ", "))
	b, _, err := compiler.CompileWithDebugInfo("foo", strings.NewReader(src))
	require.NoError(t, err)

	v := ic.SpawnVM()
	v.LoadScriptWithFlags(b, callflag.All)
	require.NoError(t, v.Run())
	require.True(t, called)
	if tc.isVoid {
		require.Equal(t, 0, v.Estack().Len())
	} else {
		require.Equal(t, 1, v.Estack().Len())
		require.Equal(t, big.NewInt(42), v.Estack().Pop().Value())
	}
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
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		var a = runtime.CheckWitness([]byte("5T"))
		func Main() bool {
			return a
		}`
	v, s := vmAndCompileInterop(t, src)
	s.interops[interopnames.ToID([]byte(interopnames.SystemRuntimeCheckWitness))] = func(v *vm.VM) error {
		s := v.Estack().Pop().Value().([]byte)
		require.Equal(t, "5T", string(s))
		v.Estack().PushVal(true)
		return nil
	}
	require.NoError(t, v.Run())
	require.Equal(t, true, v.Estack().Pop().Value())
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
