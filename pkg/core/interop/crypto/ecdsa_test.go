package crypto

import (
	"encoding/binary"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initCHECKMULTISIG(msg []byte, n int) ([]stackitem.Item, []stackitem.Item, map[string]*keys.PublicKey, error) {
	var err error

	keyMap := make(map[string]*keys.PublicKey)
	pkeys := make([]*keys.PrivateKey, n)
	pubs := make([]stackitem.Item, n)
	for i := range pubs {
		pkeys[i], err = keys.NewPrivateKey()
		if err != nil {
			return nil, nil, nil, err
		}

		pk := pkeys[i].PublicKey()
		data := pk.Bytes()
		pubs[i] = stackitem.NewByteArray(data)
		keyMap[string(data)] = pk
	}

	sigs := make([]stackitem.Item, n)
	for i := range sigs {
		sig := pkeys[i].Sign(msg)
		sigs[i] = stackitem.NewByteArray(sig)
	}

	return pubs, sigs, keyMap, nil
}

func subSlice(arr []stackitem.Item, indices []int) []stackitem.Item {
	if indices == nil {
		return arr
	}

	result := make([]stackitem.Item, len(indices))
	for i, j := range indices {
		result[i] = arr[j]
	}

	return result
}

func initCHECKMULTISIGVM(t *testing.T, n int, ik, is []int) *vm.VM {
	buf := make([]byte, 5)
	buf[0] = byte(opcode.SYSCALL)
	binary.LittleEndian.PutUint32(buf[1:], ecdsaSecp256r1CheckMultisigID)

	v := vm.New()
	v.GasLimit = -1
	ic := &interop.Context{Trigger: trigger.Verification}
	v.RegisterInteropGetter(GetInterop(ic))
	v.LoadScript(buf)
	msg := []byte("NEO - An Open Network For Smart Economy")

	pubs, sigs, _, err := initCHECKMULTISIG(msg, n)
	require.NoError(t, err)

	pubs = subSlice(pubs, ik)
	sigs = subSlice(sigs, is)

	v.Estack().PushVal(sigs)
	v.Estack().PushVal(pubs)
	v.Estack().PushVal(msg)

	return v
}

func testCHECKMULTISIGGood(t *testing.T, n int, is []int) {
	v := initCHECKMULTISIGVM(t, n, nil, is)

	require.NoError(t, v.Run())
	assert.Equal(t, 1, v.Estack().Len())
	assert.True(t, v.Estack().Pop().Bool())
}

func TestCHECKMULTISIGGood(t *testing.T) {
	t.Run("3_1", func(t *testing.T) { testCHECKMULTISIGGood(t, 3, []int{1}) })
	t.Run("2_2", func(t *testing.T) { testCHECKMULTISIGGood(t, 2, []int{0, 1}) })
	t.Run("3_3", func(t *testing.T) { testCHECKMULTISIGGood(t, 3, []int{0, 1, 2}) })
	t.Run("3_2", func(t *testing.T) { testCHECKMULTISIGGood(t, 3, []int{0, 2}) })
	t.Run("4_2", func(t *testing.T) { testCHECKMULTISIGGood(t, 4, []int{0, 2}) })
	t.Run("10_7", func(t *testing.T) { testCHECKMULTISIGGood(t, 10, []int{2, 3, 4, 5, 6, 8, 9}) })
	t.Run("12_9", func(t *testing.T) { testCHECKMULTISIGGood(t, 12, []int{0, 1, 4, 5, 6, 7, 8, 9}) })
}

func testCHECKMULTISIGBad(t *testing.T, isErr bool, n int, ik, is []int) {
	v := initCHECKMULTISIGVM(t, n, ik, is)

	if isErr {
		require.Error(t, v.Run())
		return
	}
	require.NoError(t, v.Run())
	assert.Equal(t, 1, v.Estack().Len())
	assert.False(t, v.Estack().Pop().Bool())
}

func TestCHECKMULTISIGBad(t *testing.T) {
	t.Run("1_1 wrong signature", func(t *testing.T) { testCHECKMULTISIGBad(t, false, 2, []int{0}, []int{1}) })
	t.Run("3_2 wrong order", func(t *testing.T) { testCHECKMULTISIGBad(t, false, 3, []int{0, 2}, []int{2, 0}) })
	t.Run("3_2 duplicate sig", func(t *testing.T) { testCHECKMULTISIGBad(t, false, 3, nil, []int{0, 0}) })
	t.Run("1_2 too many signatures", func(t *testing.T) { testCHECKMULTISIGBad(t, true, 2, []int{0}, []int{0, 1}) })
	t.Run("gas limit exceeded", func(t *testing.T) {
		v := initCHECKMULTISIGVM(t, 1, []int{0}, []int{0})
		v.GasLimit = ECDSAVerifyPrice - 1
		require.Error(t, v.Run())
	})
}
