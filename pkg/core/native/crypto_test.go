package native

import (
	"encoding/hex"
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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

func TestCryptoLibVerifyWithECDsa(t *testing.T) {
	t.Run("R1", func(t *testing.T) {
		testECDSAVerify(t, Secp256r1)
	})
	t.Run("K1", func(t *testing.T) {
		testECDSAVerify(t, Secp256k1)
	})
}

func testECDSAVerify(t *testing.T, curve NamedCurve) {
	var (
		priv   *keys.PrivateKey
		err    error
		c      = newCrypto()
		ic     = &interop.Context{VM: vm.New()}
		actual stackitem.Item
	)
	switch curve {
	case Secp256k1:
		priv, err = keys.NewSecp256k1PrivateKey()
	case Secp256r1:
		priv, err = keys.NewPrivateKey()
	default:
		t.Fatal("unknown curve")
	}
	require.NoError(t, err)

	runCase := func(t *testing.T, isErr bool, result interface{}, args ...interface{}) {
		argsArr := make([]stackitem.Item, len(args))
		for i := range args {
			argsArr[i] = stackitem.Make(args[i])
		}
		if isErr {
			require.Panics(t, func() {
				_ = c.verifyWithECDsa(ic, argsArr)
			})
		} else {
			require.NotPanics(t, func() {
				actual = c.verifyWithECDsa(ic, argsArr)
			})
			require.Equal(t, stackitem.Make(result), actual)
		}
	}

	msg := []byte("test message")
	sign := priv.Sign(msg)

	t.Run("bad message item", func(t *testing.T) {
		runCase(t, true, false, stackitem.NewInterop("cheburek"), priv.PublicKey().Bytes(), sign, int64(curve))
	})
	t.Run("bad pubkey item", func(t *testing.T) {
		runCase(t, true, false, msg, stackitem.NewInterop("cheburek"), sign, int64(curve))
	})
	t.Run("bad pubkey bytes", func(t *testing.T) {
		runCase(t, true, false, msg, []byte{1, 2, 3}, sign, int64(curve))
	})
	t.Run("bad signature item", func(t *testing.T) {
		runCase(t, true, false, msg, priv.PublicKey().Bytes(), stackitem.NewInterop("cheburek"), int64(curve))
	})
	t.Run("bad curve item", func(t *testing.T) {
		runCase(t, true, false, msg, priv.PublicKey().Bytes(), sign, stackitem.NewInterop("cheburek"))
	})
	t.Run("bad curve value", func(t *testing.T) {
		runCase(t, true, false, msg, priv.PublicKey().Bytes(), sign, new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(1)))
	})
	t.Run("unknown curve", func(t *testing.T) {
		runCase(t, true, false, msg, priv.PublicKey().Bytes(), sign, int64(123))
	})
	t.Run("invalid signature", func(t *testing.T) {
		s := priv.Sign(msg)
		s[0] = ^s[0]
		runCase(t, false, false, s, priv.PublicKey().Bytes(), msg, int64(curve))
	})
	t.Run("success", func(t *testing.T) {
		runCase(t, false, true, msg, priv.PublicKey().Bytes(), sign, int64(curve))
	})
}
