package compiler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeGen_DebugInfo(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop"
	import "github.com/nspcc-dev/neo-go/pkg/interop/storage"
	import "github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
var staticVar int
func init() {
	a := 1
	_ = a
}
func init() {
	x := ""
	_ = x
	staticVar = 1
}
func Main(op string) bool {
	var s string
	_ = s
	res := MethodInt(op)
	_ = MethodString()
	_ = MethodByteArray()
	_ = MethodArray()
	_ = MethodStruct()
	_ = MethodConcat("a", "b", "c")
	_ = unexportedMethod()
	return res == 42
}

func MethodInt(a string) int {
	if a == "get42" {
		return 42
	}
	return 3
}
func MethodConcat(a, b string, c string) string{
	return a + b + c
}
func MethodString() string { return "" }
func MethodByteArray() []byte { return nil }
func MethodArray() []bool { return nil }
func MethodStruct() struct{} { return struct{}{} }
func unexportedMethod() int { return 1 }
func MethodParams(addr interop.Hash160, h interop.Hash256,
	sig interop.Signature, pub interop.PublicKey,
	inter interop.Interface,
	ctx storage.Context, tx ledger.Transaction) bool {
	return true
}
type MyStruct struct {}
func (ms MyStruct) MethodOnStruct() { }
func (ms *MyStruct) MethodOnPointerToStruct() { }
func _deploy(data interface{}, isUpdate bool) { x := 1; _ = x }
`

	info, err := getBuildInfo("foo.go", src)
	require.NoError(t, err)

	pkg := info.program.Package(info.initialPackage)
	c := newCodegen(info, pkg)
	require.NoError(t, c.compile(info, pkg))

	buf, err := c.writeJumps(c.prog.Bytes())
	require.NoError(t, err)

	d := c.emitDebugInfo(buf)
	require.NotNil(t, d)

	t.Run("return types", func(t *testing.T) {
		returnTypes := map[string]string{
			"MethodInt":    "Integer",
			"MethodConcat": "ByteString",
			"MethodString": "ByteString", "MethodByteArray": "ByteString",
			"MethodArray": "Array", "MethodStruct": "Struct",
			"Main":                    "Boolean",
			"unexportedMethod":        "Integer",
			"MethodOnStruct":          "Void",
			"MethodOnPointerToStruct": "Void",
			"MethodParams":            "Boolean",
			"_deploy":                 "Void",
			manifest.MethodInit:       "Void",
		}
		for i := range d.Methods {
			name := d.Methods[i].ID
			assert.Equal(t, returnTypes[name], d.Methods[i].ReturnType)
		}
	})

	t.Run("variables", func(t *testing.T) {
		vars := map[string][]string{
			"Main":                {"s,ByteString", "res,Integer"},
			manifest.MethodInit:   {"a,Integer", "x,ByteString"},
			manifest.MethodDeploy: {"x,Integer"},
		}
		for i := range d.Methods {
			v, ok := vars[d.Methods[i].ID]
			if ok {
				require.Equal(t, v, d.Methods[i].Variables)
			}
		}
	})

	t.Run("static variables", func(t *testing.T) {
		require.Equal(t, []string{"staticVar,Integer"}, d.StaticVariables)
	})

	t.Run("param types", func(t *testing.T) {
		paramTypes := map[string][]DebugParam{
			"_deploy": {
				{
					Name:   "data",
					Type:   "Any",
					TypeSC: smartcontract.AnyType,
				},
				{
					Name:   "isUpdate",
					Type:   "Boolean",
					TypeSC: smartcontract.BoolType,
				},
			},
			"MethodInt": {{
				Name:   "a",
				Type:   "ByteString",
				TypeSC: smartcontract.StringType,
			}},
			"MethodConcat": {
				{
					Name:   "a",
					Type:   "ByteString",
					TypeSC: smartcontract.StringType,
				},
				{
					Name:   "b",
					Type:   "ByteString",
					TypeSC: smartcontract.StringType,
				},
				{
					Name:   "c",
					Type:   "ByteString",
					TypeSC: smartcontract.StringType,
				},
			},
			"Main": {{
				Name:   "op",
				Type:   "ByteString",
				TypeSC: smartcontract.StringType,
			}},
		}
		for i := range d.Methods {
			v, ok := paramTypes[d.Methods[i].ID]
			if ok {
				require.Equal(t, v, d.Methods[i].Parameters)
			}
		}
	})

	// basic check that last instruction of every method is indeed RET
	for i := range d.Methods {
		index := d.Methods[i].Range.End
		require.True(t, int(index) < len(buf))
		require.EqualValues(t, opcode.RET, buf[index])
	}

	t.Run("convert to Manifest", func(t *testing.T) {
		p := manifest.NewPermission(manifest.PermissionWildcard)
		p.Methods.Add("randomMethod")

		actual, err := d.ConvertToManifest(&Options{
			Name:        "MyCTR",
			SafeMethods: []string{"methodInt", "methodString"},
			Permissions: []manifest.Permission{*p},
		})
		require.NoError(t, err)
		expected := &manifest.Manifest{
			Name: "MyCTR",
			ABI: manifest.ABI{
				Methods: []manifest.Method{
					{
						Name:       manifest.MethodInit,
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.VoidType,
					},
					{
						Name: "_deploy",
						Parameters: []manifest.Parameter{
							manifest.NewParameter("data", smartcontract.AnyType),
							manifest.NewParameter("isUpdate", smartcontract.BoolType),
						},
						ReturnType: smartcontract.VoidType,
					},
					{
						Name: "main",
						Parameters: []manifest.Parameter{
							manifest.NewParameter("op", smartcontract.StringType),
						},
						ReturnType: smartcontract.BoolType,
					},
					{
						Name: "methodInt",
						Parameters: []manifest.Parameter{
							{
								Name: "a",
								Type: smartcontract.StringType,
							},
						},
						ReturnType: smartcontract.IntegerType,
						Safe:       true,
					},
					{
						Name:       "methodString",
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.StringType,
						Safe:       true,
					},
					{
						Name:       "methodByteArray",
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.ByteArrayType,
					},
					{
						Name:       "methodArray",
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.ArrayType,
					},
					{
						Name:       "methodStruct",
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.ArrayType,
					},
					{
						Name: "methodConcat",
						Parameters: []manifest.Parameter{
							{
								Name: "a",
								Type: smartcontract.StringType,
							},
							{
								Name: "b",
								Type: smartcontract.StringType,
							},
							{
								Name: "c",
								Type: smartcontract.StringType,
							},
						},
						ReturnType: smartcontract.StringType,
					},
					{
						Name: "methodParams",
						Parameters: []manifest.Parameter{
							manifest.NewParameter("addr", smartcontract.Hash160Type),
							manifest.NewParameter("h", smartcontract.Hash256Type),
							manifest.NewParameter("sig", smartcontract.SignatureType),
							manifest.NewParameter("pub", smartcontract.PublicKeyType),
							manifest.NewParameter("inter", smartcontract.InteropInterfaceType),
							manifest.NewParameter("ctx", smartcontract.InteropInterfaceType),
							manifest.NewParameter("tx", smartcontract.ArrayType),
						},
						ReturnType: smartcontract.BoolType,
					},
				},
				Events: []manifest.Event{},
			},
			Groups:      []manifest.Group{},
			Permissions: []manifest.Permission{*p},
			Trusts: manifest.WildPermissionDescs{
				Value: []manifest.PermissionDesc{},
			},
			Extra: json.RawMessage("null"),
		}
		require.Equal(t, len(expected.ABI.Methods), len(actual.ABI.Methods))
		for _, exp := range expected.ABI.Methods {
			md := actual.ABI.GetMethod(exp.Name, len(exp.Parameters))
			require.NotNil(t, md)
			require.Equal(t, exp.Name, md.Name)
			require.Equal(t, exp.Parameters, md.Parameters)
			require.Equal(t, exp.ReturnType, md.ReturnType)
			require.Equal(t, exp.Safe, md.Safe)
		}
		require.Equal(t, expected.ABI.Events, actual.ABI.Events)
		require.Equal(t, expected.Groups, actual.Groups)
		require.Equal(t, expected.Permissions, actual.Permissions)
		require.Equal(t, expected.Trusts, actual.Trusts)
		require.Equal(t, expected.Extra, actual.Extra)
	})
}

func TestSequencePoints(t *testing.T) {
	src := `package foo
	func Main(op string) bool {
		if op == "123" {
			return true
		}
		return false
	}`

	info, err := getBuildInfo("foo.go", src)
	require.NoError(t, err)

	pkg := info.program.Package(info.initialPackage)
	c := newCodegen(info, pkg)
	require.NoError(t, c.compile(info, pkg))

	buf := c.prog.Bytes()
	d := c.emitDebugInfo(buf)
	require.NotNil(t, d)

	require.Equal(t, d.Documents, []string{"foo.go"})

	// Main func has 2 return on 4-th and 6-th lines.
	ps := d.Methods[0].SeqPoints
	require.Equal(t, 2, len(ps))
	require.Equal(t, 4, ps[0].StartLine)
	require.Equal(t, 6, ps[1].StartLine)
}

func TestDebugInfo_MarshalJSON(t *testing.T) {
	d := &DebugInfo{
		Documents: []string{"/path/to/file"},
		Methods: []MethodDebugInfo{
			{
				ID: "id1",
				Name: DebugMethodName{
					Namespace: "default",
					Name:      "method1",
				},
				Range: DebugRange{Start: 10, End: 20},
				Parameters: []DebugParam{
					{Name: "param1", Type: "Integer"},
					{Name: "ok", Type: "Boolean"},
				},
				ReturnType: "ByteString",
				Variables:  []string{},
				SeqPoints: []DebugSeqPoint{
					{
						Opcode:    123,
						Document:  1,
						StartLine: 2,
						StartCol:  3,
						EndLine:   4,
						EndCol:    5,
					},
				},
			},
		},
		Events: []EventDebugInfo{},
	}

	testserdes.MarshalUnmarshalJSON(t, d, new(DebugInfo))
}

func TestManifestOverload(t *testing.T) {
	src := `package foo
	func Main() int {
		return 1
	}
	func Add3() int {
		return Add3Aux(0)
	}
	func Add3Aux(a int) int {
		return a + 3
	}
	func Add3Aux2(b int) int {
		return b + 3
	}
	func Add4() int {
		return 4
	}`

	_, di, err := CompileWithDebugInfo("foo", strings.NewReader(src))
	require.NoError(t, err)

	m, err := di.ConvertToManifest(&Options{Overloads: map[string]string{"add3Aux": "add3"}})
	require.NoError(t, err)
	require.NoError(t, m.ABI.IsValid())
	require.NotNil(t, m.ABI.GetMethod("add3", 0))
	require.NotNil(t, m.ABI.GetMethod("add3", 1))
	require.Nil(t, m.ABI.GetMethod("add3Aux", 1))

	t.Run("missing method", func(t *testing.T) {
		_, err := di.ConvertToManifest(&Options{Overloads: map[string]string{"miss": "add3"}})
		require.Error(t, err)
	})
	t.Run("parameter conflict", func(t *testing.T) {
		_, err := di.ConvertToManifest(&Options{Overloads: map[string]string{"add4": "add3"}})
		require.Error(t, err)
	})
	t.Run("parameter conflict, overload", func(t *testing.T) {
		_, err := di.ConvertToManifest(&Options{Overloads: map[string]string{
			"add3Aux":  "add3",
			"add3Aux2": "add3",
		}})
		require.Error(t, err)
	})
	t.Run("missing target method", func(t *testing.T) {
		_, err := di.ConvertToManifest(&Options{Overloads: map[string]string{"add4": "add5"}})
		require.Error(t, err)
	})
}
