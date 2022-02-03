package compiler_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestDefer(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer f()
			return 1
		}
		func f() { a += 2 }`
		eval(t, src, big.NewInt(3))
	})
	t.Run("ValueUnchanged", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			defer f()
			a = 3
			return a
		}
		func f() { a += 2 }`
		eval(t, src, big.NewInt(3))
	})
	t.Run("Function", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer f()
			a = 3
			return g()
		}
		func g() int {
			a++
			return a
		}
		func f() { a += 2 }`
		eval(t, src, big.NewInt(10))
	})
	t.Run("DeferAfterInterop", func(t *testing.T) {
		src := `package main

		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/storage"
		)

		func Main() {
			defer func() {
			}()
			storage.GetContext()
		}`
		vm := vmAndCompile(t, src)
		err := vm.Run()
		require.NoError(t, err)
		require.Equal(t, 0, vm.Estack().Len(), "stack contains unexpected items")
	})

	t.Run("MultipleDefers", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer f()
			defer g()
			a = 3
			return a
		}
		func g() { a *= 2 }
		func f() { a += 2 }`
		eval(t, src, big.NewInt(11))
	})
	t.Run("FunctionLiteral", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer func() {
				a = 10
			}()
			a = 3
			return a
		}`
		eval(t, src, big.NewInt(13))
	})
	t.Run("NoReturnReturn", func(t *testing.T) {
		src := `package main
		var i int
		func Main() {
			defer func() {
				i++
			}()
			return
		}`
		vm := vmAndCompile(t, src)
		err := vm.Run()
		require.NoError(t, err)
		require.Equal(t, 0, vm.Estack().Len(), "stack contains unexpected items")
	})
	t.Run("NoReturnNoReturn", func(t *testing.T) {
		src := `package main
		var i int
		func Main() {
			defer func() {
				i++
			}()
		}`
		vm := vmAndCompile(t, src)
		err := vm.Run()
		require.NoError(t, err)
		require.Equal(t, 0, vm.Estack().Len(), "stack contains unexpected items")
	})
	t.Run("CodeDuplication", func(t *testing.T) {
		src := `package main
		var i int
		func Main() {
			defer func() {
				var j int
				i += j
			}()
			if i == 1 { return }
			if i == 2 { return }
			if i == 3 { return }
			if i == 4 { return }
			if i == 5 { return }
		}`
		checkCallCount(t, src, 0 /* defer body + Main */, 2, -1)
	})
}

func TestConditionalDefer(t *testing.T) {
	type testCase struct {
		a      []bool
		result int64
	}

	t.Run("no panic", func(t *testing.T) {
		src := `package foo
		var i int
		func Main(a []bool) int { return f(a[0], a[1], a[2]) + i }
		func g() { i += 10 }
		func f(a bool, b bool, c bool) int {
			if a { defer func() { i += 1 }() }
			if b { defer g() }
			if c { defer func() { i += 100 }() }
			return 0
		}`
		testCases := []testCase{
			{[]bool{false, false, false}, 0},
			{[]bool{false, false, true}, 100},
			{[]bool{false, true, false}, 10},
			{[]bool{false, true, true}, 110},
			{[]bool{true, false, false}, 1},
			{[]bool{true, false, true}, 101},
			{[]bool{true, true, false}, 11},
			{[]bool{true, true, true}, 111},
		}
		for _, tc := range testCases {
			args := []stackitem.Item{stackitem.Make(tc.a[0]), stackitem.Make(tc.a[1]), stackitem.Make(tc.a[2])}
			evalWithArgs(t, src, nil, args, big.NewInt(tc.result))
		}
	})
	t.Run("panic between ifs", func(t *testing.T) {
		src := `package foo
		var i int
		func Main(a []bool) int { if a[1] { defer func() { recover() }() }; return f(a[0], a[1]) + i }
		func f(a, b bool) int {
			if a { defer func() { i += 1; recover() }() }
			panic("totally expected")
			if b { defer func() { i += 100; recover() }() }
			return 0
		}`

		args := []stackitem.Item{stackitem.Make(false), stackitem.Make(false)}
		v := vmAndCompile(t, src)
		v.Estack().PushVal(args)
		err := v.Run()
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "totally expected"))

		testCases := []testCase{
			{[]bool{false, true}, 0},
			{[]bool{true, false}, 1},
			{[]bool{true, true}, 1},
		}
		for _, tc := range testCases {
			args := []stackitem.Item{stackitem.Make(tc.a[0]), stackitem.Make(tc.a[1])}
			evalWithArgs(t, src, nil, args, big.NewInt(tc.result))
		}
	})
	t.Run("panic in conditional", func(t *testing.T) {
		src := `package foo
		var i int
		func Main(a []bool) int { if a[1] { defer func() { recover() }() }; return f(a[0], a[1]) + i }
		func f(a, b bool) int {
			if a {
				defer func() { i += 1; recover() }()
				panic("somewhat expected")
			}
			if b { defer func() { i += 100; recover() }() }
			return 0
		}`

		testCases := []testCase{
			{[]bool{false, false}, 0},
			{[]bool{false, true}, 100},
			{[]bool{true, false}, 1},
			{[]bool{true, true}, 1},
		}
		for _, tc := range testCases {
			args := []stackitem.Item{stackitem.Make(tc.a[0]), stackitem.Make(tc.a[1])}
			evalWithArgs(t, src, nil, args, big.NewInt(tc.result))
		}
	})
}

func TestRecover(t *testing.T) {
	t.Run("Panic", func(t *testing.T) {
		src := `package foo
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer func() {
				if r := recover(); r != nil {
					a = 3
				} else {
					a = 4
				}
			}()
			a = 1
			panic("msg")
			return a
		}`
		eval(t, src, big.NewInt(3))
	})
	t.Run("NoPanic", func(t *testing.T) {
		src := `package foo
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer func() {
				if r := recover(); r != nil {
					a = 3
				} else {
					a = 4
				}
			}()
			a = 1
			return a
		}`
		eval(t, src, big.NewInt(5))
	})
	t.Run("PanicInDefer", func(t *testing.T) {
		src := `package foo
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer func() { a += 2; recover() }()
			defer func() { a *= 3; recover(); panic("again") }()
			a = 1
			panic("msg")
			return a
		}`
		eval(t, src, big.NewInt(5))
	})
}

func TestDeferNoGlobals(t *testing.T) {
	src := `package foo
	func Main() int {
		a := 1
		defer func() { recover() }()
		panic("msg")
		return a
	}`
	eval(t, src, big.NewInt(0))
}
