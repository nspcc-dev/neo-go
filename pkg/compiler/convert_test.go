package compiler_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type convertTestCase struct {
	returnType string
	argValue   string
	result     any
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
		_, err := fmt.Fprintf(srcBuilder, srcTmpl, i, tc.returnType, tc.argValue, name)
		require.NoError(t, err)
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	for i, tc := range convertTestCases {
		v := vm.New()
		t.Run(tc.argValue+getFunctionName(tc.returnType), func(t *testing.T) {
			v.Reset(trigger.Application)
			invokeMethod(t, fmt.Sprintf("F%d", i), ne.Script, v, di)
			runAndCheck(t, v, tc.result)
		})
	}
}

func TestTypeAssertion(t *testing.T) {
	t.Run("inside return statement", func(t *testing.T) {
		src := `package foo
				func Main() int {
					a := []byte{1}
					var u any
					u = a
					return u.(int)
				}`
		eval(t, src, big.NewInt(1))
	})
	t.Run("inside general declaration", func(t *testing.T) {
		src := `package foo
				func Main() int {
					a := []byte{1}
					var u any
					u = a
					var ret = u.(int)
					return ret
				}`
		eval(t, src, big.NewInt(1))
	})
	t.Run("inside assignment statement", func(t *testing.T) {
		src := `package foo
				func Main() int {
					a := []byte{1}
					var u any
					u = a
					var ret int
					ret = u.(int)
					return ret
				}`
		eval(t, src, big.NewInt(1))
	})
	t.Run("inside definition statement", func(t *testing.T) {
		src := `package foo
				func Main() int {
					a := []byte{1}
					var u any
					u = a
					ret := u.(int)
					return ret
				}`
		eval(t, src, big.NewInt(1))
	})
}

func TestTypeAssertionWithOK(t *testing.T) {
	t.Run("inside general declaration", func(t *testing.T) {
		src := `package foo
				func Main() bool {
					a := 1
					var u any
					u = a
					var _, ok = u.(int)	// *ast.GenDecl
					return ok
				}`
		_, _, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
		require.Error(t, err)
		require.ErrorIs(t, err, compiler.ErrUnsupportedTypeAssertion)
	})
	t.Run("inside assignment statement", func(t *testing.T) {
		src := `package foo
				func Main() bool {
					a := 1
					var u any
					u = a
					var ok bool
					_, ok = u.(int)	// *ast.AssignStmt
					return ok
				}`
		_, _, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
		require.Error(t, err)
		require.ErrorIs(t, err, compiler.ErrUnsupportedTypeAssertion)
	})
	t.Run("inside definition statement", func(t *testing.T) {
		src := `package foo
				func Main() bool {
					a := 1
					var u any
					u = a
					_, ok := u.(int)	// *ast.AssignStmt
					return ok
				}`
		_, _, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
		require.Error(t, err)
		require.ErrorIs(t, err, compiler.ErrUnsupportedTypeAssertion)
	})
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
		b := any(a).(int)
		return b
	}`
	eval(t, src, big.NewInt(1))
}

func TestConvert_Uint(t *testing.T) {
	nums := []uint64{
		0,
		1,
		127,
		128,
		255,
		256,
		1024,
		1<<16 - 1,
		1 << 16,
		1<<32 - 1,
		1 << 32,
		1<<64 - 1,
	}

	var (
		countUint64 = len(nums)
		countUint32 = len(nums) - 2
		countUint16 = len(nums) - 4
		countUint8  = 5
	)

	testCases := []struct {
		nums       []uint64
		offset     int
		typ        string
		helperType string
		endians    map[string]func([]byte, uint64)
	}{
		{
			nums:       nums[:countUint64],
			typ:        "uint64",
			helperType: "Uint64",
			endians: map[string]func([]byte, uint64){
				"LE": binary.LittleEndian.PutUint64,
				"BE": binary.BigEndian.PutUint64,
			},
		},
		{
			nums:       nums[:countUint32],
			offset:     4,
			typ:        "uint32",
			helperType: "Uint32",
			endians: map[string]func([]byte, uint64){
				"LE": binary.LittleEndian.PutUint64,
				"BE": binary.BigEndian.PutUint64,
			},
		},
		{
			nums:       nums[:countUint16],
			offset:     6,
			typ:        "uint16",
			helperType: "Uint16",
			endians: map[string]func([]byte, uint64){
				"LE": binary.LittleEndian.PutUint64,
				"BE": binary.BigEndian.PutUint64,
			},
		},
		{
			nums:       nums[:countUint8],
			offset:     7,
			typ:        "uint8",
			helperType: "Uint8",
			endians: map[string]func([]byte, uint64){
				"": binary.LittleEndian.PutUint64,
			},
		},
	}

	srcToBytesTmpl := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/convert"
	
		func Main(args []any) []byte {
			return convert.%sToBytes%s(args[0].(%s))
		}
	`

	srcFromBytesTmpl := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/convert"
	
		func Main(args []any) %s {
			return convert.Bytes%sTo%s(args[0].([]byte))
		}
	`

	srcCompatibilityCheckTmpl := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/convert"
		func Main(args []any) %s {
			data := convert.%sToBytes%s(args[0].(%s))
			return convert.Bytes%sTo%s(data)
		}
	`

	for _, tc := range testCases {
		for endian, convertFunc := range tc.endians {
			srcToBytes := fmt.Sprintf(srcToBytesTmpl, tc.helperType, endian, tc.typ)
			srcFromBytes := fmt.Sprintf(srcFromBytesTmpl, tc.typ, endian, tc.helperType)
			srcCompatibilityCheck := fmt.Sprintf(srcCompatibilityCheckTmpl,
				tc.typ, tc.helperType, endian, tc.typ, endian, tc.helperType,
			)

			buf := make([]byte, 8)
			data := buf[:8-tc.offset]
			if endian == "BE" {
				data = buf[tc.offset:]
			}

			for _, num := range tc.nums {
				convertFunc(buf, num)
				evalWithArgs(t, srcToBytes, nil, []stackitem.Item{stackitem.Make(num)}, data)
				evalWithArgs(t, srcFromBytes, nil, []stackitem.Item{stackitem.Make(data)},
					new(big.Int).SetUint64(num),
				)
				evalWithArgs(t, srcCompatibilityCheck, nil, []stackitem.Item{stackitem.Make(num)},
					new(big.Int).SetUint64(num),
				)
			}
		}
	}
}
