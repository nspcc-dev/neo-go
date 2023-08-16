package compiler_test

import (
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/stretchr/testify/require"
)

func TestGenericMethodReceiver(t *testing.T) {
	t.Run("star expression", func(t *testing.T) {
		src := `
		package receiver
		type Pointer[T any] struct {
			value T
		}
		func (x *Pointer[T]) Load() *T {
			return &x.value
		}
`
		_, _, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
		require.ErrorIs(t, err, compiler.ErrGenericsUnsuppored)
	})
	t.Run("ident expression", func(t *testing.T) {
		src := `
		package receiver
		type Pointer[T any] struct {
			value T
		}
		func (x Pointer[T]) Load() *T {
			return &x.value
		}
`
		_, _, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
		require.ErrorIs(t, err, compiler.ErrGenericsUnsuppored)
	})
}

func TestGenericFuncArgument(t *testing.T) {
	src := `
		package sum
		func SumInts[V int64 | int32 | int16](vals []V) V { // doesn't make sense with NeoVM, but still it's a valid go code.
			var s V
			for i := range vals {
				s += vals[i]
			}
			return s
		}
`
	_, _, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
	require.ErrorIs(t, err, compiler.ErrGenericsUnsuppored)
}
