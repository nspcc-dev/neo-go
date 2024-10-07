package compiler_test

import (
	"bytes"
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
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
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
	require.EqualValues(t, storage.Backwards, istorage.FindBackwards)
}

type syscallTestCase struct {
	method string
	params []string
	isVoid bool
}

// This test ensures that our wrappers have the necessary number of parameters
// and execute the appropriate syscall. Because of lack of typing (compared to native contracts),
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
		"contract.Call":                    {interopnames.SystemContractCall, []string{u160, `"m"`, "1", "3"}, false},
		"contract.CreateMultisigAccount":   {interopnames.SystemContractCreateMultisigAccount, []string{"1", pubs}, false},
		"contract.CreateStandardAccount":   {interopnames.SystemContractCreateStandardAccount, []string{pub}, false},
		"contract.GetCallFlags":            {interopnames.SystemContractGetCallFlags, nil, false},
		"iterator.Next":                    {interopnames.SystemIteratorNext, []string{"iterator.Iterator{}"}, false},
		"iterator.Value":                   {interopnames.SystemIteratorValue, []string{"iterator.Iterator{}"}, false},
		"runtime.BurnGas":                  {interopnames.SystemRuntimeBurnGas, []string{"1"}, true},
		"runtime.CheckWitness":             {interopnames.SystemRuntimeCheckWitness, []string{b}, false},
		"runtime.CurrentSigners":           {interopnames.SystemRuntimeCurrentSigners, nil, false},
		"runtime.GasLeft":                  {interopnames.SystemRuntimeGasLeft, nil, false},
		"runtime.GetAddressVersion":        {interopnames.SystemRuntimeGetAddressVersion, nil, false},
		"runtime.GetCallingScriptHash":     {interopnames.SystemRuntimeGetCallingScriptHash, nil, false},
		"runtime.GetEntryScriptHash":       {interopnames.SystemRuntimeGetEntryScriptHash, nil, false},
		"runtime.GetExecutingScriptHash":   {interopnames.SystemRuntimeGetExecutingScriptHash, nil, false},
		"runtime.GetInvocationCounter":     {interopnames.SystemRuntimeGetInvocationCounter, nil, false},
		"runtime.GetNetwork":               {interopnames.SystemRuntimeGetNetwork, nil, false},
		"runtime.GetNotifications":         {interopnames.SystemRuntimeGetNotifications, []string{u160}, false},
		"runtime.GetRandom":                {interopnames.SystemRuntimeGetRandom, nil, false},
		"runtime.GetScriptContainer":       {interopnames.SystemRuntimeGetScriptContainer, nil, false},
		"runtime.GetTime":                  {interopnames.SystemRuntimeGetTime, nil, false},
		"runtime.GetTrigger":               {interopnames.SystemRuntimeGetTrigger, nil, false},
		"runtime.LoadScript":               {interopnames.SystemRuntimeLoadScript, []string{b, "0", b}, false},
		"runtime.Log":                      {interopnames.SystemRuntimeLog, []string{`"msg"`}, true},
		"runtime.Notify":                   {interopnames.SystemRuntimeNotify, []string{`"ev"`, "1"}, true},
		"runtime.Platform":                 {interopnames.SystemRuntimePlatform, nil, false},
		"storage.Delete":                   {interopnames.SystemStorageDelete, []string{sctx, b}, true},
		"storage.Find":                     {interopnames.SystemStorageFind, []string{sctx, b, "storage.None"}, false},
		"storage.Get":                      {interopnames.SystemStorageGet, []string{sctx, b}, false},
		"storage.GetContext":               {interopnames.SystemStorageGetContext, nil, false},
		"storage.GetReadOnlyContext":       {interopnames.SystemStorageGetReadOnlyContext, nil, false},
		"storage.Put":                      {interopnames.SystemStoragePut, []string{sctx, b, b}, true},
		"storage.ConvertContextToReadOnly": {interopnames.SystemStorageAsReadOnly, []string{sctx}, false},
		"crypto.CheckMultisig":             {interopnames.SystemCryptoCheckMultisig, []string{pubs, sigs}, false},
		"crypto.CheckSig":                  {interopnames.SystemCryptoCheckSig, []string{pub, sig}, false},
	}
	ic := &interop.Context{}
	core.SpawnVM(ic) // set Functions field
	for _, fs := range ic.Functions {
		// It will be set in test and we want to fail if calling invalid syscall.
		fs.Func = nil
	}

	srcBuilder := bytes.NewBuffer(nil)
	srcBuilder.WriteString(`package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		import "github.com/nspcc-dev/neo-go/pkg/interop/storage"
		import "github.com/nspcc-dev/neo-go/pkg/interop/iterator"
		import "github.com/nspcc-dev/neo-go/pkg/interop/crypto"
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		func unused() { var _ interop.Hash160 }
	`)
	for goName, tc := range interops {
		realName := strings.ToTitle(strings.Replace(goName, ".", "", 1))
		var tmpl string
		if tc.isVoid {
			tmpl = "func %s() { %s(%s) }\n"
		} else {
			tmpl = "func %s() any { return %s(%s) }\n"
		}
		srcBuilder.WriteString(fmt.Sprintf(tmpl, realName, goName, strings.Join(tc.params, ", ")))
	}

	nf, di, err := compiler.CompileWithOptions("foo.go", srcBuilder, nil)
	require.NoError(t, err)

	for goName, tc := range interops {
		t.Run(goName, func(t *testing.T) {
			name := strings.ToTitle(strings.Replace(goName, ".", "", 1))
			runSyscallTestCase(t, ic, name, nf.Script, di, tc)
		})
	}
}

func runSyscallTestCase(t *testing.T, ic *interop.Context, realName string,
	script []byte, debugInfo *compiler.DebugInfo, tc syscallTestCase) {
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

	invokeMethod(t, realName, script, ic.VM, debugInfo)
	require.NoError(t, ic.VM.Run())
	require.True(t, called)
	if tc.isVoid {
		require.Equal(t, 0, ic.VM.Estack().Len())
	} else {
		require.Equal(t, 1, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
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

	v, s, _ := vmAndCompileInterop(t, src)
	v.Estack().PushVal(11)

	require.NoError(t, v.Run())
	require.Equal(t, 2, len(s.events))

	exp0 := []stackitem.Item{stackitem.NewBigInteger(big.NewInt(11)), stackitem.NewByteArray([]byte("sum")), stackitem.NewBigInteger(big.NewInt(12))}
	assert.Equal(t, "Event1", s.events[0].Name)
	assert.Equal(t, exp0, s.events[0].Item.Value())
	assert.Equal(t, "single", s.events[1].Name)
	assert.Equal(t, []stackitem.Item{}, s.events[1].Item.Value())

	t.Run("long event name", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		func Main(arg int) {
			runtime.Notify("long event12345678901234567890123")
		}`

		_, _, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
		require.Error(t, err)
	})
}

func TestSyscallInGlobalInit(t *testing.T) {
	src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		var a = runtime.CheckWitness([]byte("5T"))
		func Main() bool {
			return a
		}`
	v, s, _ := vmAndCompileInterop(t, src)
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
	t.Run("MODMUL", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() []int {
			r := make([]int, 6)
			r[0] = math.ModMul(3, 4, 5)
			r[1] = math.ModMul(-3, 4, 5)
			r[2] = math.ModMul(3, 4, -5)
			r[3] = math.ModMul(-3, 4, -5)
			r[4] = math.ModMul(0, 4, 5)
			r[5] = math.ModMul(100, -1, -91)
			return r
		}`
		eval(t, src, []stackitem.Item{
			stackitem.Make(2),
			stackitem.Make(-2),
			stackitem.Make(2),
			stackitem.Make(-2),
			stackitem.Make(0),
			stackitem.Make(-9),
		})
	})
	t.Run("MODPOW", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/math"
		func Main() []int {
			r := make([]int, 5)
			r[0] = math.ModPow(3, 4, 5)
			r[1] = math.ModPow(-3, 5, 5)
			r[2] = math.ModPow(3, 4, -5)
			r[3] = math.ModPow(-3, 5, -5)
			r[4] = math.ModPow(19, -1, 141)
			return r
		}`
		eval(t, src, []stackitem.Item{
			stackitem.Make(1),
			stackitem.Make(2),
			stackitem.Make(1),
			stackitem.Make(2),
			stackitem.Make(52),
		})
	})
}

func TestInteropTypesComparison(t *testing.T) {
	typeCheck := func(t *testing.T, typeName string, typeLen int) {
		t.Run(typeName, func(t *testing.T) {
			var ha, hb string
			for i := 0; i < typeLen; i++ {
				if i == typeLen-1 {
					ha += "2"
					hb += "3"
				} else {
					ha += "1, "
					hb += "1, "
				}
			}
			check := func(t *testing.T, a, b string, expected bool) {
				src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		func Main() bool {
			a := interop.` + typeName + `{` + a + `}
			b := interop.` + typeName + `{` + b + `}
			return a.Equals(b)
		}`
				eval(t, src, expected)
			}
			t.Run("same type", func(t *testing.T) {
				check(t, ha, ha, true)
				check(t, ha, hb, false)
			})
			t.Run("a is nil", func(t *testing.T) {
				src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop"

		func Main() bool {
			var a interop.` + typeName + `
			b := interop.` + typeName + `{` + hb + `}
			return a.Equals(b)
		}`
				eval(t, src, false)
			})
			t.Run("b is nil", func(t *testing.T) {
				src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop"

		func Main() bool {
			a := interop.` + typeName + `{` + ha + `}
			var b interop.` + typeName + `
			return a.Equals(b)
		}`
				eval(t, src, false)
			})
			t.Run("both nil", func(t *testing.T) {
				src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop"

		func Main() bool {
			var a interop.` + typeName + `
			var b interop.` + typeName + `
			return a.Equals(b)
		}`
				eval(t, src, true)
			})
			t.Run("different types", func(t *testing.T) {
				src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop"

		func Main() bool {
			a := interop.` + typeName + `{` + ha + `}
			b := 123
			return a.Equals(b)
		}`
				eval(t, src, false)
			})
			t.Run("b is Buffer", func(t *testing.T) {
				src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop"

		func Main() bool {
			a := interop.` + typeName + `{` + ha + `}
			b := []byte{` + ha + `}
			return a.Equals(b)
		}`
				eval(t, src, true)
			})
			t.Run("b is ByteString", func(t *testing.T) {
				src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop"

		func Main() bool {
			a := interop.` + typeName + `{` + ha + `}
			b := string([]byte{` + ha + `})
			return a.Equals(b)
		}`
				eval(t, src, true)
			})
			t.Run("b is compound type", func(t *testing.T) {
				src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop"

		func Main() bool {
			a := interop.` + typeName + `{` + ha + `}
			b := struct{}{}
			return a.Equals(b)
		}`
				vm, _, _ := vmAndCompileInterop(t, src)
				err := vm.Run()
				require.Error(t, err)
				require.True(t, strings.Contains(err.Error(), "invalid conversion: Struct/ByteString"), err)
			})
		})
	}
	typeCheck(t, "Hash160", util.Uint160Size)
	typeCheck(t, "Hash256", util.Uint256Size)
	typeCheck(t, "Signature", smartcontract.SignatureLen)
	typeCheck(t, "PublicKey", smartcontract.PublicKeyLen)
}
