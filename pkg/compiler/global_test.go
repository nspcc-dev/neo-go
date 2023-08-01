package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestUnusedGlobal(t *testing.T) {
	t.Run("simple unused", func(t *testing.T) {
		src := `package foo
				const (
					_ int = iota
					a
				)
				func Main() int {
					return 1
				}`
		prog := eval(t, src, big.NewInt(1))
		require.Equal(t, 2, len(prog)) // PUSH1 + RET
	})
	t.Run("unused with function call inside", func(t *testing.T) {
		t.Run("specification names count matches values count", func(t *testing.T) {
			src := `package foo
				var control int
				var _ = f()
				func Main() int {
					return control
				}
				func f() int {
					control = 1
					return 5
				}`
			eval(t, src, big.NewInt(1))
		})
		t.Run("specification names count differs from values count", func(t *testing.T) {
			src := `package foo
				var control int
				var _, _ = f()
				func Main() int {
					return control
				}
				func f() (int, int) {
					control = 1
					return 5, 6
				}`
			eval(t, src, big.NewInt(1))
		})
		t.Run("used", func(t *testing.T) {
			src := `package foo
				var _, A = f()
				func Main() int {
					return A
				}
				func f() (int, int) {
					return 5, 6
				}`
			eval(t, src, big.NewInt(6))
			checkInstrCount(t, src, 1, 1, 0, 0) // sslot for A, single call to f
		})
	})
	t.Run("unused without function call", func(t *testing.T) {
		src := `package foo
				var _ = 1
				var (
					_ = 2 + 3
					_, _ = 3 + 4, 5
				)
				func Main() int {
					return 1
				}`
		prog := eval(t, src, big.NewInt(1))
		require.Equal(t, 2, len(prog)) // PUSH1 + RET
	})
}

func TestUnusedOptimizedGlobalVar(t *testing.T) {
	t.Run("unused, no initialization", func(t *testing.T) {
		src := `package foo
				var A int
				var (
					B int
					C, D, E int
				)
				func Main() int {
					return 1
				}`
		prog := eval(t, src, big.NewInt(1))
		require.Equal(t, 2, len(prog)) // Main
	})
	t.Run("used, no initialization", func(t *testing.T) {
		src := `package foo
				var A int
				func Main() int {
					return A
				}`
		eval(t, src, big.NewInt(0))
		checkInstrCount(t, src, 1, 0, 0, 0) // sslot for A
	})
	t.Run("used by unused var, no initialization", func(t *testing.T) {
		src := `package foo
				var Unused int
				var Unused2 = Unused + 1
				func Main() int {
					return 1
				}`
		prog := eval(t, src, big.NewInt(1))
		require.Equal(t, 2, len(prog)) // Main
	})
	t.Run("unused, with initialization", func(t *testing.T) {
		src := `package foo
				var Unused = 1
				func Main() int {
					return 2
				}`
		prog := eval(t, src, big.NewInt(2))
		require.Equal(t, 2, len(prog)) // Main
	})
	t.Run("unused, with initialization by used var", func(t *testing.T) {
		src := `package foo
				var (
					A = 1
					B, Unused, C = f(), A + 2, 3 // the code for Unused initialization won't be emitted as it's a pure expression without function calls
					Unused2 = 4
				)
				var Unused3 = 5
				func Main() int {
					return A + C
				}
				func f() int {
					return 4
				}`
		eval(t, src, big.NewInt(4), []any{opcode.INITSSLOT, []byte{2}}, // sslot for A and C
			opcode.PUSH1, opcode.STSFLD0, // store A
			[]any{opcode.CALL, []byte{10}}, opcode.DROP, // evaluate B and drop
			opcode.PUSH3, opcode.STSFLD1, opcode.RET, // store C
			opcode.LDSFLD0, opcode.LDSFLD1, opcode.ADD, opcode.RET, // Main
			opcode.PUSH4, opcode.RET) // f
	})
	t.Run("used by unused var, with initialization", func(t *testing.T) {
		src := `package foo
				var (
					Unused1 = 1
					Unused2 = Unused1 + 1
				)
				func Main() int {
					return 1
				}`
		prog := eval(t, src, big.NewInt(1))
		require.Equal(t, 2, len(prog)) // Main
	})
	t.Run("used with combination of nested unused", func(t *testing.T) {
		src := `package foo
				var (
					A = 1
					Unused1 = 2
					Unused2 = Unused1 + 1
				)
				func Main() int {
					return A
				}`
		eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{1}}, // sslot for A
			opcode.PUSH1, opcode.STSFLD0, opcode.RET, // store A
			opcode.LDSFLD0, opcode.RET) // Main
	})
	t.Run("single var stmt with both used and unused vars", func(t *testing.T) {
		src := `package foo
				var A, Unused1, B, Unused2 = 1, 2, 3, 4
				func Main() int {
					return A + B
				}`
		eval(t, src, big.NewInt(4), []any{opcode.INITSSLOT, []byte{2}}, // sslot for A and B
			opcode.PUSH1, opcode.STSFLD0, // store A
			opcode.PUSH3, opcode.STSFLD1, opcode.RET, // store B
			opcode.LDSFLD0, opcode.LDSFLD1, opcode.ADD, opcode.RET) // Main
	})
	t.Run("single var decl token with multiple var specifications", func(t *testing.T) {
		src := `package foo
				var (
					A, Unused1, B, Unused2 = 1, 2, 3, 4
					C, Unused3 int
				)
				func Main() int {
					return A + B + C
				}`
		eval(t, src, big.NewInt(4), []any{opcode.INITSSLOT, []byte{3}}, // sslot for A, B, C
			opcode.PUSH1, opcode.STSFLD0, // store A
			opcode.PUSH3, opcode.STSFLD1, // store B
			opcode.PUSH0, opcode.STSFLD2, opcode.RET, // store C
			opcode.LDSFLD0, opcode.LDSFLD1, opcode.ADD, opcode.LDSFLD2, opcode.ADD, opcode.RET) // Main
	})
	t.Run("function as unused var value", func(t *testing.T) {
		src := `package foo
				var A, Unused1 = 1, f()
				func Main() int {
					return A
				}
				func f() int {
					return 2
				}`
		eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{1}}, // sslot for A
			opcode.PUSH1, opcode.STSFLD0, // store A
			[]any{opcode.CALL, []byte{6}}, opcode.DROP, opcode.RET, // evaluate Unused1 (call to f) and drop its value
			opcode.LDSFLD0, opcode.RET, // Main
			opcode.PUSH2, opcode.RET) // f
	})
	t.Run("function as unused struct field", func(t *testing.T) {
		src := `package foo
				type Str struct { Int int }
				var _ = Str{Int: f()}
				func Main() int {
					return 1
				}
				func f() int {
					return 2
				}`
		eval(t, src, big.NewInt(1), []any{opcode.CALL, []byte{8}}, opcode.PUSH1, opcode.PACKSTRUCT, opcode.DROP, opcode.RET, // evaluate struct val
			opcode.PUSH1, opcode.RET, // Main
			opcode.PUSH2, opcode.RET) // f
	})
	t.Run("used in unused function", func(t *testing.T) {
		src := `package foo
				var Unused1, Unused2, Unused3 = 1, 2, 3
				func Main() int {
					return 1
				}
				func unused1() int {
					return Unused1
				}
				func unused2() int {
					return Unused1 + unused1()
				}
				func unused3() int {
					return Unused2 + unused2()
				}`
		prog := eval(t, src, big.NewInt(1))
		require.Equal(t, 2, len(prog)) // Main
	})
	t.Run("used in used function", func(t *testing.T) {
		src := `package foo
				var A = 1
				func Main() int {
					return f()
				}
				func f() int {
					return A
				}`
		eval(t, src, big.NewInt(1))
		checkInstrCount(t, src, 1, 1, 0, 0)
	})
	t.Run("unused, initialized via init", func(t *testing.T) {
		src := `package foo
				var A int
				func Main() int {
					return 2
				}
				func init() {
					A = 1		// Although A is unused from exported functions, it's used from init(), so it should be mark as "used" and stored.
				}`
		eval(t, src, big.NewInt(2))
		checkInstrCount(t, src, 1, 0, 0, 0)
	})
	t.Run("used, initialized via init", func(t *testing.T) {
		src := `package foo
				var A int
				func Main() int {
					return A
				}
				func init() {
					A = 1
				}`
		eval(t, src, big.NewInt(1))
		checkInstrCount(t, src, 1, 0, 0, 0)
	})
	t.Run("unused, initialized by function call", func(t *testing.T) {
		t.Run("unnamed", func(t *testing.T) {
			src := `package foo
					var _ = f()
					func Main() int {
						return 1
					}
					func f() int {
						return 2
					}`
			eval(t, src, big.NewInt(1))
			checkInstrCount(t, src, 0, 1, 0, 0)
		})
		t.Run("named", func(t *testing.T) {
			src := `package foo
					var A = f()
					func Main() int {
						return 1
					}
					func f() int {
						return 2
					}`
			eval(t, src, big.NewInt(1))
			checkInstrCount(t, src, 0, 1, 0, 0)
		})
		t.Run("named, with dependency on unused var", func(t *testing.T) {
			src := `package foo
					var (
						A = 1
						B = A + 1 // To check nested ident values.
						C = 3
						D = B + f() + C // To check that both idents (before and after the call to f) will be marked as "used".
						E = C + 1 		// Unused, no code expected.
					)
					func Main() int {
						return 1
					}
					func f() int {
						return 2
					}`
			eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{3}}, // sslot for A
				opcode.PUSH1, opcode.STSFLD0, // store A
				opcode.LDSFLD0, opcode.PUSH1, opcode.ADD, opcode.STSFLD1, // store B
				opcode.PUSH3, opcode.STSFLD2, // store C
				opcode.LDSFLD1, []any{opcode.CALL, []byte{9}}, opcode.ADD, opcode.LDSFLD2, opcode.ADD, opcode.DROP, opcode.RET, // evaluate D and drop
				opcode.PUSH1, opcode.RET, // Main
				opcode.PUSH2, opcode.RET) // f
		})
		t.Run("named, with dependency on unused var ident inside function call", func(t *testing.T) {
			src := `package foo
					var A = 1
					var B = f(A)
					func Main() int {
						return 1
					}
					func f(a int) int {
						return a
					}`
			eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{1}}, // sslot for A
				opcode.PUSH1, opcode.STSFLD0, // store A
				opcode.LDSFLD0, []any{opcode.CALL, []byte{6}}, opcode.DROP, opcode.RET, // evaluate B and drop
				opcode.PUSH1, opcode.RET, // Main
				[]any{opcode.INITSLOT, []byte{0, 1}}, opcode.LDARG0, opcode.RET) // f
		})
		t.Run("named, inside multi-specs and multi-vals var declaration", func(t *testing.T) {
			src := `package foo
					var (
						Unused = 1
						Unused1, A, Unused2 = 2, 3 + f(), 4
					)
					func Main() int {
						return 1
					}
					func f() int {
						return 5
					}`
			eval(t, src, big.NewInt(1), opcode.PUSH3, []any{opcode.CALL, []byte{7}}, opcode.ADD, opcode.DROP, opcode.RET, // evaluate and drop A
				opcode.PUSH1, opcode.RET, // Main
				opcode.PUSH5, opcode.RET) // f
		})
		t.Run("unnamed + unused", func(t *testing.T) {
			src := `package foo
					var A = 1 // At least one global variable is used, thus, the whole set of package variables will be walked.
					var B = 2
					var _ = B + 1 // This variable is unnamed and doesn't contain call, thus its children won't be marked as "used".
					func Main() int {
						return A
					}`
			eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{1}}, //  sslot for A
				opcode.PUSH1, opcode.STSFLD0, opcode.RET, // store A
				opcode.LDSFLD0, opcode.RET) // Main
		})
		t.Run("mixed value", func(t *testing.T) {
			src := `package foo
					var control int // At least one global variable is used, thus the whole set of package variables will be walked.
					var B = 2
					var _ = 1 + f() + B // This variable is unnamed but contains call, thus its children will be marked as "used".
					func Main() int {
						return control
					}
					func f() int {
						control = 1
						return 3
					}`
			eval(t, src, big.NewInt(1))
			checkInstrCount(t, src, 2 /* control + B */, 1, 0, 0)
		})
		t.Run("multiple function return values", func(t *testing.T) {
			src := `package foo
					var A, B = f()
					func Main() int {
						return A
					}
					func f() (int, int) {
						return 3, 4
					}`
			eval(t, src, big.NewInt(3), []any{opcode.INITSSLOT, []byte{1}}, // sslot for A
				[]any{opcode.CALL, []byte{7}}, opcode.STSFLD0, opcode.DROP, opcode.RET, // evaluate and store A, drop B
				opcode.LDSFLD0, opcode.RET, // Main
				opcode.PUSH4, opcode.PUSH3, opcode.RET) // f
		})
		t.Run("constant in declaration", func(t *testing.T) {
			src := `package foo
					const A = 5
					var Unused = 1 + A
					func Main() int {
						return 1
					}`
			prog := eval(t, src, big.NewInt(1))
			require.Equal(t, 2, len(prog)) // Main
		})
		t.Run("mixed expression", func(t *testing.T) {
			src := `package foo
					type CustomInt struct {
						Int int
					}
					var A = CustomInt{Int: 2}
					var B = f(3) + A.f(1)
					func Main() int {
						return 1
					}
					func f(a int) int {
						return a
					}
					func (i CustomInt) f(a int) int {	// has the same name as f
						return i.Int + a
					}`
			eval(t, src, big.NewInt(1))
			checkInstrCount(t, src, 1 /* A */, 2, 2, 0)
		})
	})
	t.Run("mixed nested expressions", func(t *testing.T) {
		src := `package foo
				type CustomInt struct {	Int int}	// has the same field name as Int variable, important for test
				var A = CustomInt{Int: 2}
				var B = f(A.Int)
				var Unused = 4
				var Int = 5		// unused and MUST NOT be treated as "used"
				var C = CustomInt{Int: Unused}.Int + f(1) 	// uses Unused => Unused should be marked as "used"
				func Main() int {
					return 1
				}
				func f(a int) int {
					return a
				}
				func (i CustomInt) f(a int) int {	// has the same name as f
					return i.Int + a
				}`
		eval(t, src, big.NewInt(1))
	})
	t.Run("composite literal", func(t *testing.T) {
		src := `package foo
				var A = 2
				var B = []int{1, A, 3}[1]
				var C = f(1) + B
				func Main() int {
					return 1
				}
				func f(a int) int {
					return a
				}`
		eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{2}}, // sslot for A, B
			opcode.PUSH2, opcode.STSFLD0, // store A
			opcode.PUSH3, opcode.LDSFLD0, opcode.PUSH1, opcode.PUSH3, opcode.PACK, opcode.PUSH1, opcode.PICKITEM, opcode.STSFLD1, // evaluate B
			opcode.PUSH1, []any{opcode.CALL, []byte{8}}, opcode.LDSFLD1, opcode.ADD, opcode.DROP, opcode.RET, // evalute C and drop
			opcode.PUSH1, opcode.RET, // Main
			[]any{opcode.INITSLOT, []byte{0, 1}}, opcode.LDARG0, opcode.RET) // f
	})
	t.Run("index expression", func(t *testing.T) {
		src := `package foo
				var Unused = 2
				var A = f(1) + []int{1, 2, 3}[Unused] // index expression
				func Main() int {
					return 1
				}
				func f(a int) int {
					return a
				}`
		eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{1}}, // sslot for Unused
			opcode.PUSH2, opcode.STSFLD0, // store Unused
			opcode.PUSH1, []any{opcode.CALL, []byte{14}}, // call f(1)
			opcode.PUSH3, opcode.PUSH2, opcode.PUSH1, opcode.PUSH3, opcode.PACK, opcode.LDSFLD0, opcode.PICKITEM, // eval index expression
			opcode.ADD, opcode.DROP, opcode.RET, // eval and drop A
			opcode.PUSH1, opcode.RET, // Main
			[]any{opcode.INITSLOT, []byte{0, 1}}, opcode.LDARG0, opcode.RET) // f(a)
	})
	t.Run("used via nested function calls", func(t *testing.T) {
		src := `package foo
				var A = 1
				func Main() int {
					return f()
				}
				func f() int {
					return g()
				}
				func g() int {
					return A
				}`
		eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{1}}, // sslot for A
			opcode.PUSH1, opcode.STSFLD0, opcode.RET, // store A
			[]any{opcode.CALL, []byte{3}}, opcode.RET, // Main
			[]any{opcode.CALL, []byte{3}}, opcode.RET, // f
			opcode.LDSFLD0, opcode.RET) // g
	})
	t.Run("struct field name matches global var name", func(t *testing.T) {
		src := `package foo
				type CustomStr struct { Int int	}
				var str = CustomStr{Int: 2}
				var Int = 5 // Unused and the code must not be emitted.
				func Main() int {
					return str.Int
				}`
		eval(t, src, big.NewInt(2), []any{opcode.INITSSLOT, []byte{1}}, // sslot for str
			opcode.PUSH2, opcode.PUSH1, opcode.PACKSTRUCT, opcode.STSFLD0, opcode.RET, // store str
			opcode.LDSFLD0, opcode.PUSH0, opcode.PICKITEM, opcode.RET) // Main
	})
	t.Run("var as a struct field initializer", func(t *testing.T) {
		src := `package foo
				type CustomStr struct { Int int	}
				var A = 5
				var Int = 6 // Unused
				func Main() int {
					return CustomStr{Int: A}.Int
				}`
		eval(t, src, big.NewInt(5))
	})
	t.Run("argument of globally called function", func(t *testing.T) {
		src := `package foo
				var Unused = 5
				var control int
				var _, A = f(Unused)
				func Main() int {
					return control
				}
				func f(int) (int, int) {
					control = 5
					return 1, 2
				}`
		eval(t, src, big.NewInt(5))
	})
	t.Run("argument of locally called function", func(t *testing.T) {
		src := `package foo
				var Unused = 5
				func Main() int {
					var _, a = f(Unused)
					return a
				}
				func f(i int) (int, int) {
					return i, i
				}`
		eval(t, src, big.NewInt(5))
	})
	t.Run("used in globally called defer", func(t *testing.T) {
		src := `package foo
				var control1, control2 int
				var Unused = 5
				var _ = f()
				func Main() int {
					return control1 + control2
				}
				func f() int {
					control1 = 1
					defer func(){
						control2 = Unused
					}()
					return 2
				}`
		eval(t, src, big.NewInt(6))
	})
	t.Run("used in locally called defer", func(t *testing.T) {
		src := `package foo
				var control1, control2 int
				var Unused = 5
				func Main() int {
					_ = f()
					return control1 + control2
				}
				func f() int {
					control1 = 1
					defer func(){
						control2 = Unused
					}()
					return 2
				}`
		eval(t, src, big.NewInt(6))
	})
	t.Run("imported", func(t *testing.T) {
		t.Run("init by func call", func(t *testing.T) {
			src := `package foo
					import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar"
					func Main() int {
						return globalvar.Default
					}`
			eval(t, src, big.NewInt(0))
			checkInstrCount(t, src, 1 /* Default */, 1 /* f */, 0, 0)
		})
		t.Run("nested var call", func(t *testing.T) {
			src := `package foo
					import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar/nested1"
					func Main() int {
						return nested1.C
					}`
			eval(t, src, big.NewInt(81))
			checkInstrCount(t, src, 6 /* dependant vars of nested1.C */, 3, 1, 1)
		})
		t.Run("nested func call", func(t *testing.T) {
			src := `package foo
					import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar/funccall"
					func Main() int {
						return funccall.F()
					}`
			eval(t, src, big.NewInt(56))
			checkInstrCount(t, src, 2 /* nested2.Argument + nested1.Argument */, -1, -1, -1)
		})
		t.Run("nested method call", func(t *testing.T) {
			src := `package foo
					import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/globalvar/funccall"
					func Main() int {
						return funccall.GetAge()
					}`
			eval(t, src, big.NewInt(24))
			checkInstrCount(t, src, 3, /* nested3.Anna + nested2.Argument + nested3.Argument */
				5, /* funccall.GetAge() + Anna.GetAge() + nested1.f + nested1.f + nested2.f */
				2 /* nested1.f + nested2.f */, 0)
		})
	})
}

func TestChangeGlobal(t *testing.T) {
	t.Run("from Main", func(t *testing.T) {
		src := `package foo
				var a int
				func Main() int {
					setLocal()
					set42()
					setLocal()
					return a
				}
				func set42() { a = 42 }
				func setLocal() { a := 10; _ = a }`
		eval(t, src, big.NewInt(42))
	})
	t.Run("from other global", func(t *testing.T) {
		t.Skip("see https://github.com/nspcc-dev/neo-go/issues/2661")
		src := `package foo
				var A = f()
				var B int
				func Main() int {
					return B
				}
				func f() int {
					B = 3
					return B
				}`
		eval(t, src, big.NewInt(3))
	})
}

func TestMultiDeclaration(t *testing.T) {
	src := `package foo
	var a, b, c int
	func Main() int {
		a = 1
		b = 2
		c = 3
		return a + b + c
	}`
	eval(t, src, big.NewInt(6))
}

func TestCountLocal(t *testing.T) {
	src := `package foo
	func Main() int {
		a, b, c, d := f()
		return a + b + c + d
	}
	func f() (int, int, int, int) {
		return 1, 2, 3, 4
	}`
	eval(t, src, big.NewInt(10))
}

func TestMultiDeclarationLocal(t *testing.T) {
	src := `package foo
	func Main() int {
		var a, b, c int
		a = 1
		b = 2
		c = 3
		return a + b + c
	}`
	eval(t, src, big.NewInt(6))
}

func TestMultiDeclarationLocalCompound(t *testing.T) {
	src := `package foo
	func Main() int {
		var a, b, c []int
		a = append(a, 1)
		b = append(b, 2)
		c = append(c, 3)
		return a[0] + b[0] + c[0]
	}`
	eval(t, src, big.NewInt(6))
}

func TestShadow(t *testing.T) {
	srcTmpl := `package foo
	func Main() int {
		x := 1
		y := 10
		%s
			x += 1  // increase old local
			x := 30 // introduce new local
			y += x  // make sure is means something
		}
		return x+y
	}`

	runCase := func(b string) func(t *testing.T) {
		return func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, b)
			eval(t, src, big.NewInt(42))
		}
	}

	t.Run("If", runCase("if true {"))
	t.Run("For", runCase("for i := 0; i < 1; i++ {"))
	t.Run("Range", runCase("for range []int{1} {"))
	t.Run("Switch", runCase("switch true {\ncase false: x += 2\ncase true:"))
	t.Run("Block", runCase("{"))
}

func TestArgumentLocal(t *testing.T) {
	srcTmpl := `package foo
	func some(a int) int {
	    if a > 42 {
	        a := 24
			_ = a
	    }
	    return a
	}
	func Main() int {
		return some(%d)
	}`
	t.Run("Override", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, 50)
		eval(t, src, big.NewInt(50))
	})
	t.Run("NoOverride", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, 40)
		eval(t, src, big.NewInt(40))
	})
}

func TestContractWithNoMain(t *testing.T) {
	src := `package foo
	var someGlobal int = 1
	func Add3(a int) int {
		someLocal := 2
		return someGlobal + someLocal + a
	}`
	b, di, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
	require.NoError(t, err)
	v := vm.New()
	invokeMethod(t, "Add3", b.Script, v, di)
	v.Estack().PushVal(39)
	require.NoError(t, v.Run())
	require.Equal(t, 1, v.Estack().Len())
	require.Equal(t, big.NewInt(42), v.PopResult())
}

func TestMultipleFiles(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
	func Main() int {
		return multi.Sum()
	}`
	eval(t, src, big.NewInt(42))
}

func TestExportedVariable(t *testing.T) {
	t.Run("Use", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			return multi.SomeVar12
		}`
		eval(t, src, big.NewInt(12))
	})
	t.Run("ChangeAndUse", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			multi.SomeVar12 = 10
			return multi.Sum()
		}`
		eval(t, src, big.NewInt(40))
	})
	t.Run("PackageAlias", func(t *testing.T) {
		src := `package foo
		import kek "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			kek.SomeVar12 = 10
			return kek.Sum()
		}`
		eval(t, src, big.NewInt(40))
	})
	t.Run("DifferentName", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/strange"
		func Main() int {
			normal.NormalVar = 42
			return normal.NormalVar
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("MultipleEqualNames", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		var SomeVar12 = 1
		func Main() int {
			SomeVar30 := 3
			sum := SomeVar12 + multi.SomeVar30
			sum += SomeVar30
			sum += multi.SomeVar12
			return sum
		}`
		eval(t, src, big.NewInt(46))
	})
}

func TestExportedConst(t *testing.T) {
	t.Run("with vars", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			return multi.SomeConst
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("const only", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/constonly"
		func Main() int {
			return constonly.Answer
		}`
		eval(t, src, big.NewInt(42))
	})
}

func TestMultipleFuncSameName(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			return multi.Sum() + Sum()
		}
		func Sum() int {
			return 11
		}`
		eval(t, src, big.NewInt(53))
	})
	t.Run("WithMethod", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/foo"
		type Foo struct{}
		func (f Foo) Bar() int { return 11 }
		func Bar() int { return 22 }
		func Main() int {
			var a Foo
			var b foo.Foo
			return a.Bar() + // 11
				foo.Bar() +  // 1
				b.Bar() +    // 8
				Bar()        // 22
		}`
		eval(t, src, big.NewInt(42))
	})
}

func TestConstDontUseSlots(t *testing.T) {
	const count = 256
	buf := bytes.NewBufferString("package foo\n")
	for i := 0; i < count; i++ {
		buf.WriteString(fmt.Sprintf("const n%d = 1\n", i))
	}
	buf.WriteString("func Main() int { sum := 0\n")
	for i := 0; i < count; i++ {
		buf.WriteString(fmt.Sprintf("sum += n%d\n", i))
	}
	buf.WriteString("return sum }")

	src := buf.String()
	eval(t, src, big.NewInt(count))
}

func TestUnderscoreVarsDontUseSlots(t *testing.T) {
	const count = 128
	buf := bytes.NewBufferString("package foo\n")
	for i := 0; i < count; i++ {
		buf.WriteString(fmt.Sprintf("var _, n%d = 1, 1\n", i))
	}
	buf.WriteString("func Main() int { sum := 0\n")
	for i := 0; i < count; i++ {
		buf.WriteString(fmt.Sprintf("sum += n%d\n", i))
	}
	buf.WriteString("return sum }")

	src := buf.String()
	eval(t, src, big.NewInt(count))
}

func TestUnderscoreGlobalVarDontEmitCode(t *testing.T) {
	src := `package foo
		var _ int
		var _ = 1
		var (
			A = 2
			_ = A + 3
			_, B, _ = 4, 5, 6
			_, C, _ = f(A, B)
		)
		var D = 7 // named unused, after global codegen optimisation no code expected
		func Main() int {
			return 1
		}
		func f(a, b int) (int, int, int) {
			return 8, 9, 10
		}`
	eval(t, src, big.NewInt(1), []any{opcode.INITSSLOT, []byte{2}}, // sslot for A, B
		opcode.PUSH2, opcode.STSFLD0, // store A
		opcode.PUSH5, opcode.STSFLD1, // store B
		opcode.LDSFLD0, opcode.LDSFLD1, opcode.SWAP, []any{opcode.CALL, []byte{8}}, // evaluate f(A,B)
		opcode.DROP, opcode.DROP, opcode.DROP, opcode.RET, // drop result of f(A,B)
		opcode.PUSH1, opcode.RET, // Main
		[]any{opcode.INITSLOT, []byte{0, 2}}, opcode.PUSH10, opcode.PUSH9, opcode.PUSH8, opcode.RET) // f
}
