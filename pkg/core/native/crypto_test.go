package native

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestSha256(t *testing.T) {
	c := newCrypto()
	ic := &interop.Context{VM: vm.New()}

	t.Run("bad arg type", func(t *testing.T) {
		require.Panics(t, func() {
			c.sha256(ic, []stackitem.Item{stackitem.NewInterop(nil)})
		})
	})
	t.Run("good", func(t *testing.T) {
		// 0x0100 hashes to 47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254
		require.Equal(t, "47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254", hex.EncodeToString(c.sha256(ic, []stackitem.Item{stackitem.NewByteArray([]byte{1, 0})}).Value().([]byte)))
	})
}

func TestRIPEMD160(t *testing.T) {
	c := newCrypto()
	ic := &interop.Context{VM: vm.New()}

	t.Run("bad arg type", func(t *testing.T) {
		require.Panics(t, func() {
			c.ripemd160(ic, []stackitem.Item{stackitem.NewInterop(nil)})
		})
	})
	t.Run("good", func(t *testing.T) {
		// 0x0100 hashes to 213492c0c6fc5d61497cf17249dd31cd9964b8a3
		require.Equal(t, "213492c0c6fc5d61497cf17249dd31cd9964b8a3", hex.EncodeToString(c.ripemd160(ic, []stackitem.Item{stackitem.NewByteArray([]byte{1, 0})}).Value().([]byte)))
	})
}
