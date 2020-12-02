package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testVerifiable []byte

var _ crypto.Verifiable = testVerifiable{}

func (v testVerifiable) GetSignedPart() []byte {
	return v
}
func (v testVerifiable) GetSignedHash() util.Uint256 {
	return hash.Sha256(v)
}

func testHash0100(t *testing.T, result string, interopFunc func(*interop.Context) error) {
	t.Run("good", func(t *testing.T) {
		bs := []byte{1, 0}

		checkGood := func(t *testing.T, ic *interop.Context) {
			require.NoError(t, interopFunc(ic))
			require.Equal(t, 1, ic.VM.Estack().Len())
			require.Equal(t, result, hex.EncodeToString(ic.VM.Estack().Pop().Bytes()))
		}
		t.Run("raw bytes", func(t *testing.T) {
			ic := &interop.Context{VM: vm.New()}
			ic.VM.Estack().PushVal(bs)
			checkGood(t, ic)
		})
		t.Run("interop", func(t *testing.T) {
			ic := &interop.Context{VM: vm.New()}
			ic.VM.Estack().PushVal(stackitem.NewInterop(testVerifiable(bs)))
			checkGood(t, ic)
		})
		t.Run("container", func(t *testing.T) {
			ic := &interop.Context{VM: vm.New(), Container: testVerifiable(bs)}
			ic.VM.Estack().PushVal(stackitem.Null{})
			checkGood(t, ic)
		})
	})
	t.Run("bad message", func(t *testing.T) {
		ic := &interop.Context{VM: vm.New()}
		ic.VM.Estack().PushVal(stackitem.NewArray(nil))
		require.Error(t, interopFunc(ic))
	})
}

func TestSHA256(t *testing.T) {
	// 0x0100 hashes to 47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254
	res := "47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254"
	testHash0100(t, res, Sha256)
}

func TestRIPEMD160(t *testing.T) {
	// 0x0100 hashes to 213492c0c6fc5d61497cf17249dd31cd9964b8a3
	res := "213492c0c6fc5d61497cf17249dd31cd9964b8a3"
	testHash0100(t, res, RipeMD160)
}
