package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSHA256(t *testing.T) {
	// 0x0100 hashes to 47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254
	res := "47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254"
	buf := io.NewBufBinWriter()
	emit.Bytes(buf.BinWriter, []byte{1, 0})
	emit.Syscall(buf.BinWriter, "Neo.Crypto.SHA256")
	prog := buf.Bytes()
	ic := &interop.Context{Trigger: trigger.Verification}
	Register(ic)
	v := ic.SpawnVM()
	v.Load(prog)
	require.NoError(t, v.Run())
	assert.Equal(t, 1, v.Estack().Len())
	assert.Equal(t, res, hex.EncodeToString(v.Estack().Pop().Bytes()))
}

func TestRIPEMD160(t *testing.T) {
	// 0x0100 hashes to 213492c0c6fc5d61497cf17249dd31cd9964b8a3
	res := "213492c0c6fc5d61497cf17249dd31cd9964b8a3"
	buf := io.NewBufBinWriter()
	emit.Bytes(buf.BinWriter, []byte{1, 0})
	emit.Syscall(buf.BinWriter, "Neo.Crypto.RIPEMD160")
	prog := buf.Bytes()
	ic := &interop.Context{Trigger: trigger.Verification}
	Register(ic)
	v := ic.SpawnVM()
	v.Load(prog)
	require.NoError(t, v.Run())
	assert.Equal(t, 1, v.Estack().Len())
	assert.Equal(t, res, hex.EncodeToString(v.Estack().Pop().Bytes()))
}
