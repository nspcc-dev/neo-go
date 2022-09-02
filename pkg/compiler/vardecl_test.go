package compiler_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

func TestGenDeclWithMultiRet(t *testing.T) {
	t.Run("global var decl", func(t *testing.T) {
		src := `package foo
				func Main() int {
					var a, b = f()
					return a + b
				}
				func f() (int, int) {
					return 1, 2
				}`
		eval(t, src, big.NewInt(3))
	})
	t.Run("local var decl", func(t *testing.T) {
		src := `package foo
				var a, b = f()
				func Main() int {
					return a + b
				}
				func f() (int, int) {
					return 1, 2
				}`
		eval(t, src, big.NewInt(3))
	})
}

func TestUnderscoreLocalVarDontEmitCode(t *testing.T) {
	src := `package foo
		type Foo struct { Int int }
		func Main() int {
			var _ int
			var _ = 1
			var (
				A = 2
				_ = A + 3
				_, B, _ = 4, 5, 6
				_, _, _ = f(A, B) // unused, but has function call, so the code is expected
				_, C, _ = f(A, B)
			)
			var D = 7 // unused but named, so the code is expected
			_ = D
			var _ = Foo{ Int: 5 }
			var fo = Foo{ Int: 3 }
			var _ = 1 + A + fo.Int
			var _ = fo.GetInt()	// unused, but has method call, so the code is expected
			return C
		}
		func f(a, b int) (int, int, int) {
			return 8, 9, 10
		}
		func (fo Foo) GetInt() int {
			return fo.Int
		}`
	eval(t, src, big.NewInt(9), []interface{}{opcode.INITSLOT, []byte{5, 0}}, // local slot for A, B, C, D, fo
		opcode.PUSH2, opcode.STLOC0, // store A
		opcode.PUSH5, opcode.STLOC1, // store B
		opcode.LDLOC0, opcode.LDLOC1, opcode.SWAP, []interface{}{opcode.CALL, []byte{27}}, // evaluate f() first time
		opcode.DROP, opcode.DROP, opcode.DROP, // drop all values from f
		opcode.LDLOC0, opcode.LDLOC1, opcode.SWAP, []interface{}{opcode.CALL, []byte{19}}, // evaluate f() second time
		opcode.DROP, opcode.STLOC2, opcode.DROP, // store C
		opcode.PUSH7, opcode.STLOC3, // store D
		opcode.LDLOC3, opcode.DROP, // empty assignment
		opcode.PUSH3, opcode.PUSH1, opcode.PACKSTRUCT, opcode.STLOC4, // fo decl
		opcode.LDLOC4, []interface{}{opcode.CALL, []byte{12}}, opcode.DROP, // fo.GetInt()
		opcode.LDLOC2, opcode.RET, // return C
		[]interface{}{opcode.INITSLOT, []byte{0, 2}}, opcode.PUSH10, opcode.PUSH9, opcode.PUSH8, opcode.RET, // f
		[]interface{}{opcode.INITSLOT, []byte{0, 1}}, opcode.LDARG0, opcode.PUSH0, opcode.PICKITEM, opcode.RET) // (fo Foo) GetInt() int
}
