package compiler

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeGen_DebugInfo(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/interop"
	import "github.com/nspcc-dev/neo-go/pkg/interop/storage"
	import "github.com/nspcc-dev/neo-go/pkg/interop/blockchain"
	import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
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
	inter interop.Interface, ctr contract.Contract,
	ctx storage.Context, tx blockchain.Transaction) bool {
	return true
}
type MyStruct struct {}
func (ms MyStruct) MethodOnStruct() { }
func (ms *MyStruct) MethodOnPointerToStruct() { }
func _deploy(isUpdate bool) {}
`

	info, err := getBuildInfo("foo.go", src)
	require.NoError(t, err)

	pkg := info.program.Package(info.initialPackage)
	c := newCodegen(info, pkg)
	require.NoError(t, c.compile(info, pkg))

	buf := c.prog.Bytes()
	d := c.emitDebugInfo(buf)
	require.NotNil(t, d)

	t.Run("hash", func(t *testing.T) {
		require.True(t, hash.Hash160(buf).Equals(d.Hash))
	})

	t.Run("return types", func(t *testing.T) {
		returnTypes := map[string]string{
			"MethodInt":    "Integer",
			"MethodConcat": "String",
			"MethodString": "String", "MethodByteArray": "ByteString",
			"MethodArray": "Array", "MethodStruct": "Struct",
			"Main":                    "Boolean",
			"unexportedMethod":        "Integer",
			"MethodOnStruct":          "Void",
			"MethodOnPointerToStruct": "Void",
			"MethodParams":            "Boolean",
			"_deploy":                 "Void",
		}
		for i := range d.Methods {
			name := d.Methods[i].ID
			assert.Equal(t, returnTypes[name], d.Methods[i].ReturnType)
		}
	})

	t.Run("variables", func(t *testing.T) {
		vars := map[string][]string{
			"Main": {"s,String", "res,Integer"},
		}
		for i := range d.Methods {
			v, ok := vars[d.Methods[i].ID]
			if ok {
				require.Equal(t, v, d.Methods[i].Variables)
			}
		}
	})

	t.Run("param types", func(t *testing.T) {
		paramTypes := map[string][]DebugParam{
			"_deploy": {{
				Name: "isUpdate",
				Type: "Boolean",
			}},
			"MethodInt": {{
				Name: "a",
				Type: "String",
			}},
			"MethodConcat": {
				{
					Name: "a",
					Type: "String",
				},
				{
					Name: "b",
					Type: "String",
				},
				{
					Name: "c",
					Type: "String",
				},
			},
			"Main": {{
				Name: "op",
				Type: "String",
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
		actual, err := d.ConvertToManifest("MyCTR", nil)
		require.NoError(t, err)
		// note: offsets are hard to predict, so we just take them from the output
		expected := &manifest.Manifest{
			Name: "MyCTR",
			ABI: manifest.ABI{
				Hash: hash.Hash160(buf),
				Methods: []manifest.Method{
					{
						Name:   "_deploy",
						Offset: 0,
						Parameters: []manifest.Parameter{
							manifest.NewParameter("isUpdate", smartcontract.BoolType),
						},
						ReturnType: smartcontract.VoidType,
					},
					{
						Name:   "main",
						Offset: 4,
						Parameters: []manifest.Parameter{
							manifest.NewParameter("op", smartcontract.StringType),
						},
						ReturnType: smartcontract.BoolType,
					},
					{
						Name:   "methodInt",
						Offset: 70,
						Parameters: []manifest.Parameter{
							{
								Name: "a",
								Type: smartcontract.StringType,
							},
						},
						ReturnType: smartcontract.IntegerType,
					},
					{
						Name:       "methodString",
						Offset:     101,
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.StringType,
					},
					{
						Name:       "methodByteArray",
						Offset:     107,
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.ByteArrayType,
					},
					{
						Name:       "methodArray",
						Offset:     112,
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.ArrayType,
					},
					{
						Name:       "methodStruct",
						Offset:     117,
						Parameters: []manifest.Parameter{},
						ReturnType: smartcontract.ArrayType,
					},
					{
						Name:   "methodConcat",
						Offset: 92,
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
						Name:   "methodParams",
						Offset: 129,
						Parameters: []manifest.Parameter{
							manifest.NewParameter("addr", smartcontract.Hash160Type),
							manifest.NewParameter("h", smartcontract.Hash256Type),
							manifest.NewParameter("sig", smartcontract.SignatureType),
							manifest.NewParameter("pub", smartcontract.PublicKeyType),
							manifest.NewParameter("inter", smartcontract.InteropInterfaceType),
							manifest.NewParameter("ctr", smartcontract.ArrayType),
							manifest.NewParameter("ctx", smartcontract.InteropInterfaceType),
							manifest.NewParameter("tx", smartcontract.ArrayType),
						},
						ReturnType: smartcontract.BoolType,
					},
				},
				Events: []manifest.Event{},
			},
			Groups: []manifest.Group{},
			Permissions: []manifest.Permission{
				{
					Contract: manifest.PermissionDesc{
						Type: manifest.PermissionWildcard,
					},
					Methods: manifest.WildStrings{},
				},
			},
			Trusts: manifest.WildUint160s{
				Value: []util.Uint160{},
			},
			SafeMethods: manifest.WildStrings{
				Value: []string{},
			},
			Extra: nil,
		}
		require.True(t, expected.ABI.Hash.Equals(actual.ABI.Hash))
		require.ElementsMatch(t, expected.ABI.Methods, actual.ABI.Methods)
		require.Equal(t, expected.ABI.Events, actual.ABI.Events)
		require.Equal(t, expected.Groups, actual.Groups)
		require.Equal(t, expected.Permissions, actual.Permissions)
		require.Equal(t, expected.Trusts, actual.Trusts)
		require.Equal(t, expected.SafeMethods, actual.SafeMethods)
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
		Hash:      util.Uint160{10, 11, 12, 13},
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
					{"param1", "Integer"},
					{"ok", "Boolean"},
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
