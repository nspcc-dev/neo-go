package compiler_test

import (
	"fmt"
	"math/big"
	"testing"
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
		return "ByteArray"
	case "int64":
		return "Integer"
	}
	panic("invalid type")
}

func TestConvert(t *testing.T) {
	srcTmpl := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop/convert"
	func Main() %s {
		arg := %s
		return convert.To%s(arg)
	}`

	convertTestCases := []convertTestCase{
		{"bool", "true", true},
		{"bool", "false", false},
		{"bool", "12", true},
		{"bool", "0", false},
		{"bool", "[]byte{0, 1, 0}", true},
		{"bool", "[]byte{0}", false},
		{"int64", "true", big.NewInt(1)},
		{"int64", "false", big.NewInt(0)},
		{"int64", "12", big.NewInt(12)},
		{"int64", "0", big.NewInt(0)},
		{"int64", "[]byte{0, 1, 0}", big.NewInt(256)},
		{"int64", "[]byte{0}", big.NewInt(0)},
		{"[]byte", "true", []byte{1}},
		{"[]byte", "false", []byte{}},
		{"[]byte", "12", []byte{0x0C}},
		{"[]byte", "0", []byte{}},
		{"[]byte", "[]byte{0, 1, 0}", []byte{0, 1, 0}},
	}

	for _, tc := range convertTestCases {
		name := getFunctionName(tc.returnType)
		t.Run(tc.argValue+"->"+name, func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, tc.returnType, tc.argValue, name)
			eval(t, src, tc.result)
		})
	}
}
