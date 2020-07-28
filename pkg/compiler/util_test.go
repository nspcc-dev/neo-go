package compiler_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/stretchr/testify/require"
)

func TestSHA256(t *testing.T) {
	src := `
		package foo
		 import (
			"github.com/nspcc-dev/neo-go/pkg/interop/crypto"
		 )
		func Main() []byte {
			src := []byte{0x97}
			hash := crypto.SHA256(src)
			return hash
		}
	`
	v := vmAndCompile(t, src)
	ic := &interop.Context{Trigger: trigger.Verification}
	crypto.Register(ic)
	v.SyscallHandler = ic.SyscallHandler
	require.NoError(t, v.Run())
	require.True(t, v.Estack().Len() >= 1)

	h := []byte{0x2a, 0xa, 0xb7, 0x32, 0xb4, 0xe9, 0xd8, 0x5e, 0xf7, 0xdc, 0x25, 0x30, 0x3b, 0x64, 0xab, 0x52, 0x7c, 0x25, 0xa4, 0xd7, 0x78, 0x15, 0xeb, 0xb5, 0x79, 0xf3, 0x96, 0xec, 0x6c, 0xac, 0xca, 0xd3}
	require.Equal(t, h, v.PopResult())
}
