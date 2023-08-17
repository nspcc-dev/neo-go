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
