package compiler

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeGen_DebugInfo(t *testing.T) {
	src := `package foo
func Main(op string) bool {
	var s string
	_ = s
	res := methodInt(op)
	_ = methodString()	
	_ = methodByteArray()
	_ = methodArray()
	_ = methodStruct()
	return res == 42
}

func methodInt(a string) int {
	if a == "get42" {
		return 42
	}
	return 3
}
func methodConcat(a, b string, c string) string{
	return a + b + c
}
func methodString() string { return "" }
func methodByteArray() []byte { return nil }
func methodArray() []bool { return nil }
func methodStruct() struct{} { return struct{}{} }
`

	info, err := getBuildInfo(src)
	require.NoError(t, err)

	pkg := info.program.Package(info.initialPackage)
	c := newCodegen(info, pkg)
	require.NoError(t, c.compile(info, pkg))

	buf := c.prog.Bytes()
	d := c.emitDebugInfo()
	require.NotNil(t, d)

	t.Run("return types", func(t *testing.T) {
		returnTypes := map[string]string{
			"methodInt":    "Integer",
			"methodConcat": "String",
			"methodString": "String", "methodByteArray": "ByteArray",
			"methodArray": "Array", "methodStruct": "Struct",
			"Main": "Boolean",
		}
		for i := range d.Methods {
			name := d.Methods[i].Name.Name
			assert.Equal(t, returnTypes[name], d.Methods[i].ReturnType)
		}
	})

	t.Run("variables", func(t *testing.T) {
		vars := map[string][]string{
			"Main": {"s,String", "res,Integer"},
		}
		for i := range d.Methods {
			v, ok := vars[d.Methods[i].Name.Name]
			if ok {
				require.Equal(t, v, d.Methods[i].Variables)
			}
		}
	})

	t.Run("param types", func(t *testing.T) {
		paramTypes := map[string][]DebugParam{
			"methodInt": {{
				Name: "a",
				Type: "String",
			}},
			"methodConcat": {
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
			v, ok := paramTypes[d.Methods[i].Name.Name]
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

	t.Run("convert to ABI", func(t *testing.T) {
		actual := d.convertToABI(buf, smartcontract.HasStorage)
		expected := ABI{
			Hash: hash.Hash160(buf),
			Metadata: Metadata{
				HasStorage:           true,
				HasDynamicInvocation: false,
				IsPayable:            false,
			},
			EntryPoint: mainIdent,
			Functions: []Method{
				{
					Name: mainIdent,
					Parameters: []DebugParam{
						{
							Name: "op",
							Type: "String",
						},
					},
					ReturnType: "Boolean",
				},
			},
			Events: []Event{},
		}
		assert.Equal(t, expected, actual)
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

	info, err := getBuildInfo(src)
	require.NoError(t, err)

	pkg := info.program.Package(info.initialPackage)
	c := newCodegen(info, pkg)
	require.NoError(t, c.compile(info, pkg))

	d := c.emitDebugInfo()
	require.NotNil(t, d)

	// Main func has 2 return on 4-th and 6-th lines.
	ps := d.Methods[0].SeqPoints
	require.Equal(t, 2, len(ps))
	require.Equal(t, 4, ps[0].StartLine)
	require.Equal(t, 6, ps[1].StartLine)
}

func TestDebugInfo_MarshalJSON(t *testing.T) {
	d := &DebugInfo{
		EntryPoint: "main",
		Documents:  []string{"/path/to/file"},
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
				ReturnType: "ByteArray",
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
