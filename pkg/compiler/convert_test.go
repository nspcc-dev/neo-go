package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

type convertTestCase struct {
	returnType string
	argValue   string
	result     interface{}
}

func getFunctionName(typ string) string {
	switch typ {
	case "bool":
		return "Bool"
	case "[]byte":
		return "Bytes"
	case "int":
		return "Integer"
	}
	panic("invalid type")
}

func TestConvert(t *testing.T) {
	srcTmpl := `func F%d() %s {
		arg := %s
		return convert.To%s(arg)
	}
	`

	convertTestCases := []convertTestCase{
		{"bool", "true", true},
		{"bool", "false", false},
		{"bool", "12", true},
		{"bool", "0", false},
		{"bool", "[]byte{0, 1, 0}", true},
		{"bool", "[]byte{0}", true},
		{"bool", `""`, false},
		{"int", "true", big.NewInt(1)},
		{"int", "false", big.NewInt(0)},
		{"int", "12", big.NewInt(12)},
		{"int", "0", big.NewInt(0)},
		{"int", "[]byte{0, 1, 0}", big.NewInt(256)},
		{"int", "[]byte{0}", big.NewInt(0)},
		{"[]byte", "true", []byte{1}},
		{"[]byte", "false", []byte{0}},
		{"[]byte", "12", []byte{0x0C}},
		{"[]byte", "0", []byte{}},
		{"[]byte", "[]byte{0, 1, 0}", []byte{0, 1, 0}},
	}

	srcBuilder := bytes.NewBuffer([]byte(`package testcase
		import "github.com/nspcc-dev/neo-go/pkg/interop/convert"
	`))
	for i, tc := range convertTestCases {
		name := getFunctionName(tc.returnType)
		srcBuilder.WriteString(fmt.Sprintf(srcTmpl, i, tc.returnType, tc.argValue, name))
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	for i, tc := range convertTestCases {
		v := vm.New()
		t.Run(tc.argValue+getFunctionName(tc.returnType), func(t *testing.T) {
			v.Istack().Clear()
			v.Estack().Clear()
			invokeMethod(t, fmt.Sprintf("F%d", i), ne.Script, v, di)
			runAndCheck(t, v, tc.result)
		})
	}
}

func TestTypeAssertion(t *testing.T) {
	src := `package foo
	func Main() int {
		a := []byte{1}
		var u interface{}
		u = a
		return u.(int)
	}`
	eval(t, src, big.NewInt(1))
}

func TestTypeConversion(t *testing.T) {
	src := `package foo
	type myInt int
	func Main() int32 {
		var a int32 = 41
		b := myInt(a)
		incMy := func(x myInt) myInt { return x + 1 }
		c := incMy(b)
		return int32(c)
	}`

	eval(t, src, big.NewInt(42))
}

func TestSelectorTypeConversion(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/types"
	import "github.com/nspcc-dev/neo-go/pkg/interop/util"
	import "github.com/nspcc-dev/neo-go/pkg/interop"
	func Main() int {
		var a int
		if util.Equals(types.Buffer(nil), nil) {
			a += 1
		}

	    // Buffer != ByteArray
		if util.Equals(types.Buffer("\x12"), "\x12") {
			a += 10
		}

		tmp := []byte{0x23}
		if util.Equals(types.ByteString(tmp), "\x23") {
			a += 100
		}

		addr := "aaaaaaaaaaaaaaaaaaaa"
		buf := []byte(addr)
		if util.Equals(interop.Hash160(addr), interop.Hash160(buf)) {
			a += 1000
		}
		return a
	}`
	eval(t, src, big.NewInt(1101))
}

func TestTypeConversionString(t *testing.T) {
	src := `package foo
	type mystr string
	func Main() mystr {
		b := []byte{'l', 'a', 'm', 'a', 'o'}
		s := mystr(b)
		b[0] = 'u'
		return s
	}`
	eval(t, src, []byte("lamao"))
}

func TestInterfaceTypeConversion(t *testing.T) {
	src := `package foo
	func Main() int {
		a := 1
		b := interface{}(a).(int)
		return b
	}`
	eval(t, src, big.NewInt(1))
}
