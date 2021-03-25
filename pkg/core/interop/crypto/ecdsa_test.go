package crypto

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initCHECKMULTISIG(msgHash util.Uint256, n int) ([]stackitem.Item, []stackitem.Item, map[string]*keys.PublicKey, error) {
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
		sig := pkeys[i].SignHash(msgHash)
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

func initCheckMultisigVMNoArgs(container *transaction.Transaction) *vm.VM {
	buf := make([]byte, 5)
	buf[0] = byte(opcode.SYSCALL)
	binary.LittleEndian.PutUint32(buf[1:], neoCryptoCheckMultisigID)

	ic := &interop.Context{
		Network:   uint32(netmode.UnitTestNet),
		Trigger:   trigger.Verification,
		Container: container,
	}
	Register(ic)
	v := ic.SpawnVM()
	v.LoadScript(buf)
	return v
}

func initCHECKMULTISIGVM(t *testing.T, n int, ik, is []int) *vm.VM {
	tx := transaction.New([]byte("NEO - An Open Network For Smart Economy"), 10)
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Scripts = []transaction.Witness{{}}

	v := initCheckMultisigVMNoArgs(tx)

	pubs, sigs, _, err := initCHECKMULTISIG(hash.NetSha256(uint32(netmode.UnitTestNet), tx), n)
	require.NoError(t, err)

	pubs = subSlice(pubs, ik)
	sigs = subSlice(sigs, is)

	v.Estack().PushVal(sigs)
	v.Estack().PushVal(pubs)

	return v
}

func testCHECKMULTISIGGood(t *testing.T, n int, is []int) {
	v := initCHECKMULTISIGVM(t, n, nil, is)

	require.NoError(t, v.Run())
	assert.Equal(t, 1, v.Estack().Len())
	assert.True(t, v.Estack().Pop().Bool())
}

func TestECDSASecp256r1CheckMultisigGood(t *testing.T) {
	testCurveCHECKMULTISIGGood(t)
}

func testCurveCHECKMULTISIGGood(t *testing.T) {
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

func TestECDSASecp256r1CheckMultisigBad(t *testing.T) {
	testCurveCHECKMULTISIGBad(t)
}

func testCurveCHECKMULTISIGBad(t *testing.T) {
	t.Run("1_1 wrong signature", func(t *testing.T) { testCHECKMULTISIGBad(t, false, 2, []int{0}, []int{1}) })
	t.Run("3_2 wrong order", func(t *testing.T) { testCHECKMULTISIGBad(t, false, 3, []int{0, 2}, []int{2, 0}) })
	t.Run("3_2 duplicate sig", func(t *testing.T) { testCHECKMULTISIGBad(t, false, 3, nil, []int{0, 0}) })
	t.Run("1_2 too many signatures", func(t *testing.T) { testCHECKMULTISIGBad(t, true, 2, []int{0}, []int{0, 1}) })
	t.Run("gas limit exceeded", func(t *testing.T) {
		v := initCHECKMULTISIGVM(t, 1, []int{0}, []int{0})
		v.GasLimit = fee.ECDSAVerifyPrice - 1
		require.Error(t, v.Run())
	})

	msg := []byte("NEO - An Open Network For Smart Economy")
	pubs, sigs, _, err := initCHECKMULTISIG(hash.Sha256(msg), 1)
	require.NoError(t, err)
	arr := stackitem.NewArray([]stackitem.Item{stackitem.NewArray(nil)})
	tx := transaction.New([]byte("NEO - An Open Network For Smart Economy"), 10)
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Scripts = []transaction.Witness{{}}

	t.Run("invalid public keys", func(t *testing.T) {
		v := initCheckMultisigVMNoArgs(tx)
		v.Estack().PushVal(sigs)
		v.Estack().PushVal(arr)
		require.Error(t, v.Run())
	})
	t.Run("invalid signatures", func(t *testing.T) {
		v := initCheckMultisigVMNoArgs(tx)
		v.Estack().PushVal(arr)
		v.Estack().PushVal(pubs)
		require.Error(t, v.Run())
	})
}

func TestCheckSig(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	verifyFunc := ECDSASecp256r1CheckSig
	d := dao.NewSimple(storage.NewMemoryStore(), false)
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), DAO: dao.NewCached(d)}
	runCase := func(t *testing.T, isErr bool, result interface{}, args ...interface{}) {
		ic.SpawnVM()
		for i := range args {
			ic.VM.Estack().PushVal(args[i])
		}

		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			err = verifyFunc(ic)
		}()

		if isErr {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, 1, ic.VM.Estack().Len())
		require.Equal(t, result, ic.VM.Estack().Pop().Value().(bool))
	}

	tx := transaction.New([]byte{0, 1, 2}, 1)
	ic.Container = tx

	t.Run("success", func(t *testing.T) {
		sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
		runCase(t, false, true, sign, priv.PublicKey().Bytes())
	})

	t.Run("missing argument", func(t *testing.T) {
		runCase(t, true, false)
		sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
		runCase(t, true, false, sign)
	})

	t.Run("invalid signature", func(t *testing.T) {
		sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
		sign[0] = ^sign[0]
		runCase(t, false, false, sign, priv.PublicKey().Bytes())
	})

	t.Run("invalid public key", func(t *testing.T) {
		sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)
		pub := priv.PublicKey().Bytes()
		pub[0] = 0xFF // invalid prefix
		runCase(t, true, false, sign, pub)
	})
}
