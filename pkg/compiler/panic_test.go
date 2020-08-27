package compiler_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPanic(t *testing.T) {
	t.Run("no panic", func(t *testing.T) {
		src := getPanicSource(false, `"execution fault"`)
		eval(t, src, big.NewInt(7))
	})

	t.Run("panic with message", func(t *testing.T) {
		src := getPanicSource(true, `"execution fault"`)
		v := vmAndCompile(t, src)

		require.Error(t, v.Run())
		require.True(t, v.HasFailed())
	})

	t.Run("panic with nil", func(t *testing.T) {
		src := getPanicSource(true, `nil`)
		v := vmAndCompile(t, src)

		require.Error(t, v.Run())
		require.True(t, v.HasFailed())
	})
}

func getPanicSource(need bool, message string) string {
	return fmt.Sprintf(`
	package main
	func Main() int {
		needPanic := %#v
		if needPanic {
			panic(%s)
			return 5
		}
		return 7
	}
	`, need, message)
}
