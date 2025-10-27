package native

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"slices"
	"testing"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
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

// TestKeccak256_Compat is a C# node compatibility test with data taken from https://github.com/Jim8y/neo/blob/560d35783e428d31e3681eaa7ee9ed00a8a50d09/tests/Neo.UnitTests/SmartContract/Native/UT_CryptoLib.cs#L340
func TestKeccak256_Compat(t *testing.T) {
	c := newCrypto()
	ic := &interop.Context{VM: vm.New()}

	t.Run("good", func(t *testing.T) {
		testCases := []struct {
			name         string
			input        []byte
			expectedHash string
		}{
			{"good", []byte{1, 0}, "628bf3596747d233f1e6533345700066bf458fa48daedaf04a7be6c392902476"},
			{"hello world", []byte("Hello, World!"), "acaf3289d7b601cbd114fb36c4d29c85bbfd5e133f14cb355c3fd8d99367964f"},
			{"keccak", []byte("Keccak"), "868c016b666c7d3698636ee1bd023f3f065621514ab61bf26f062c175fdbe7f2"},
			{"cryptography", []byte("Cryptography"), "53d49d225dd2cfe77d8c5e2112bcc9efe77bea1c7aa5e5ede5798a36e99e2d29"},
			{"testing123", []byte("Testing123"), "3f82db7b16b0818a1c6b2c6152e265f682d5ebcf497c9aad776ad38bc39cb6ca"},
			{"long string", []byte("This is a longer string for Keccak256 testing purposes."), "24115e5c2359f85f6840b42acd2f7ea47bc239583e576d766fa173bf711bdd2f"},
			{"blank string", []byte(""), "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := c.keccak256(ic, []stackitem.Item{stackitem.NewByteArray(tc.input)}).Value().([]byte)
				outputHashHex := hex.EncodeToString(result)
				require.Equal(t, tc.expectedHash, outputHashHex)
			})
		}
	})
	t.Run("errors", func(t *testing.T) {
		errCases := []struct {
			name string
			item stackitem.Item
		}{
			{
				name: "Null item",
				item: stackitem.Null{},
			},
			{
				name: "not a byte array",
				item: stackitem.NewArray([]stackitem.Item{stackitem.NewBool(true)}),
			},
		}

		for _, tc := range errCases {
			t.Run(tc.name, func(t *testing.T) {
				require.Panics(t, func() {
					_ = c.keccak256(ic, []stackitem.Item{tc.item})
				}, "keccak256 should panic with incorrect argument types")
			})
		}
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

func TestMurmur32(t *testing.T) {
	c := newCrypto()
	ic := &interop.Context{VM: vm.New()}

	t.Run("bad arg type", func(t *testing.T) {
		require.Panics(t, func() {
			c.murmur32(ic, []stackitem.Item{stackitem.NewInterop(nil), stackitem.Make(5)})
		})
	})
	t.Run("good", func(t *testing.T) {
		// Example from the C# node:
		// https://github.com/neo-project/neo/blob/2a64c1cc809d1ff4b3a573c7c22bffbbf69a738b/tests/neo.UnitTests/Cryptography/UT_Murmur32.cs#L18
		data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1}
		seed := 10
		expected := make([]byte, 4)
		binary.LittleEndian.PutUint32(expected, 378574820)
		require.Equal(t, expected, c.murmur32(ic, []stackitem.Item{stackitem.NewByteArray(data), stackitem.Make(seed)}).Value().([]byte))
	})
}

func TestCryptoLibVerifyWithECDsa(t *testing.T) {
	t.Run("R1 sha256", func(t *testing.T) {
		testECDSAVerify(t, Secp256r1Sha256)
	})
	t.Run("K1 sha256", func(t *testing.T) {
		testECDSAVerify(t, Secp256k1Sha256)
	})
	t.Run("R1 keccak256", func(t *testing.T) {
		testECDSAVerify(t, Secp256r1Keccak256)
	})
	t.Run("K1 keccak256", func(t *testing.T) {
		testECDSAVerify(t, Secp256k1Keccak256)
	})
}

func testECDSAVerify(t *testing.T, curve NamedCurveHash) {
	var (
		priv   *keys.PrivateKey
		err    error
		c      = newCrypto()
		ic     = &interop.Context{VM: vm.New()}
		actual stackitem.Item
		hasher HashFunc
	)
	switch curve {
	case Secp256k1Sha256:
		priv, err = keys.NewSecp256k1PrivateKey()
		hasher = hash.Sha256
	case Secp256r1Sha256:
		priv, err = keys.NewPrivateKey()
		hasher = hash.Sha256
	case Secp256k1Keccak256:
		priv, err = keys.NewSecp256k1PrivateKey()
		hasher = Keccak256
	case Secp256r1Keccak256:
		priv, err = keys.NewPrivateKey()
		hasher = Keccak256
	default:
		t.Fatal("unknown curve/hash")
	}
	require.NoError(t, err)

	runCase := func(t *testing.T, isErr bool, result any, args ...any) {
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
	sign := priv.SignHash(hasher(msg))

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
		runCase(t, true, false, msg, priv.PublicKey().Bytes(), sign, int64(124))
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

func TestCryptoLib_VerifyWithED25519(t *testing.T) {
	var (
		c      = newCrypto()
		ic     = &interop.Context{VM: vm.New()}
		actual stackitem.Item
		msg    = []byte("The quick brown fox jumps over the lazy dog")
	)

	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	sig, err := priv.Sign(nil, msg, &ed25519.Options{})
	require.NoError(t, err)

	runCase := func(t *testing.T, isErr bool, expected any, args ...any) {
		argsArr := make([]stackitem.Item, len(args))
		for i := range args {
			argsArr[i] = stackitem.Make(args[i])
		}
		if isErr {
			require.Panics(t, func() {
				_ = c.verifyWithEd25519(ic, argsArr)
			})
		} else {
			require.NotPanics(t, func() {
				actual = c.verifyWithEd25519(ic, argsArr)
			})
			require.Equal(t, stackitem.Make(expected), actual)
		}
	}

	t.Run("bad message item", func(t *testing.T) {
		runCase(t, true, false, stackitem.NewInterop("cheburek"), []byte(pub), sig)
	})
	t.Run("bad pubkey item", func(t *testing.T) {
		runCase(t, true, false, msg, stackitem.NewInterop("cheburek"), sig)
	})
	t.Run("bad pubkey bytes", func(t *testing.T) {
		runCase(t, false, false, msg, []byte{1, 2, 3}, sig)
	})
	t.Run("bad signature item", func(t *testing.T) {
		runCase(t, true, false, msg, []byte(pub), stackitem.NewInterop("cheburek"))
	})
	t.Run("bad signature bytes", func(t *testing.T) {
		runCase(t, false, false, msg, []byte(pub), []byte{1, 2, 3})
	})
	t.Run("invalid signature", func(t *testing.T) {
		cp := slices.Clone(sig)
		cp[0] = ^cp[0]
		runCase(t, false, false, cp, []byte(pub), msg)
	})
	t.Run("success", func(t *testing.T) {
		runCase(t, false, true, msg, []byte(pub), sig)
	})
}

// TestCryptoLib_RecoverSecp256K1 uses raw test data from
// https://github.com/neo-project/neo/blob/76a968b6620f6cdaba461f482f9e84bd3a5953ac/tests/Neo.UnitTests/Cryptography/UT_Crypto.cs#L97.
func TestCryptoLib_RecoverSecp256K1(t *testing.T) {
	var (
		c      = newCrypto()
		ic     = &interop.Context{VM: vm.New()}
		actual stackitem.Item
	)

	msgH, err := hex.DecodeString("5ae8317d34d1e595e3fa7247db80c0af4320cce1116de187f8f7e2e099c0d8d0")
	require.NoError(t, err)
	sig, err := hex.DecodeString("45c0b7f8c09a9e1f1cea0c25785594427b6bf8f9f878a8af0b1abbb48e16d0920d8becd0c220f67c51217eecfd7184ef0732481c843857e6bc7fc095c4f6b78801")
	require.NoError(t, err)
	pub, err := hex.DecodeString("034a071e8a6e10aada2b8cf39fa3b5fb3400b04e99ea8ae64ceea1a977dbeaf5d5")
	require.NoError(t, err)

	runCase := func(t *testing.T, isErr bool, expected any, args ...any) {
		argsArr := make([]stackitem.Item, len(args))
		for i := range args {
			argsArr[i] = stackitem.Make(args[i])
		}
		if isErr {
			require.Panics(t, func() {
				_ = c.recoverSecp256K1(ic, argsArr)
			})
		} else {
			require.NotPanics(t, func() {
				actual = c.recoverSecp256K1(ic, argsArr)
			})
			require.Equal(t, stackitem.Make(expected), actual)
		}
	}

	t.Run("bad message hash item", func(t *testing.T) {
		runCase(t, true, false, stackitem.NewInterop("cheburek"), sig)
	})
	t.Run("bad signature item", func(t *testing.T) {
		runCase(t, true, false, msgH, stackitem.NewInterop("cheburek"))
	})
	t.Run("invalid message hash len", func(t *testing.T) {
		runCase(t, false, stackitem.Null{}, []byte{1, 2, 3}, sig)
	})
	t.Run("bad signature len", func(t *testing.T) {
		runCase(t, false, stackitem.Null{}, msgH, []byte{1, 2, 3})
	})
	t.Run("invalid signature", func(t *testing.T) {
		cp := slices.Clone(sig)
		cp[1] = ^cp[1]
		runCase(t, false, stackitem.Null{}, cp, msgH, sig)
	})
	t.Run("success", func(t *testing.T) {
		runCase(t, false, pub, msgH, sig)
	})
}

func TestCryptolib_ScalarFromBytes_Compat(t *testing.T) {
	r2Ref := &fr.Element{
		0xc999_e990_f3f2_9c6d,
		0x2b6c_edcb_8792_5c23,
		0x05d3_1496_7254_398f,
		0x0748_d9d9_9f59_ff11,
	} // R2 Scalar representation taken from the https://github.com/neo-project/Neo.Cryptography.BLS12_381/blob/844bc3a4f7d8ba2c545ace90ca124f8ada4c8d29/src/Neo.Cryptography.BLS12_381/ScalarConstants.cs#L55

	tcs := map[string]struct {
		bytes      []byte
		expected   *fr.Element
		shouldFail bool
	}{
		"zero": {
			bytes:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expected: new(fr.Element).SetZero(),
		},
		"one": {
			bytes:    []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expected: new(fr.Element).SetOne(),
		},
		"R2": {
			bytes:    []byte{254, 255, 255, 255, 1, 0, 0, 0, 2, 72, 3, 0, 250, 183, 132, 88, 245, 79, 188, 236, 239, 79, 140, 153, 111, 5, 197, 172, 89, 177, 36, 24},
			expected: r2Ref,
		},
		"negative": {
			bytes: []byte{0, 0, 0, 0, 255, 255, 255, 255, 254, 91, 254, 255, 2, 164, 189, 83, 5, 216, 161, 9, 8, 216, 57, 51, 72, 125, 157, 41, 83, 167, 237, 115},
		},
		"modulus": {
			bytes:      []byte{1, 0, 0, 0, 255, 255, 255, 255, 254, 91, 254, 255, 2, 164, 189, 83, 5, 216, 161, 9, 8, 216, 57, 51, 72, 125, 157, 41, 83, 167, 237, 115},
			shouldFail: true,
		},
		"larger than modulus": {
			bytes:      []byte{2, 0, 0, 0, 255, 255, 255, 255, 254, 91, 254, 255, 2, 164, 189, 83, 5, 216, 161, 9, 8, 216, 57, 51, 72, 125, 157, 41, 83, 167, 237, 115},
			shouldFail: true,
		},
		"larger than modulus 2": {
			bytes:      []byte{1, 0, 0, 0, 255, 255, 255, 255, 254, 91, 254, 255, 2, 164, 189, 83, 5, 216, 161, 9, 8, 216, 58, 51, 72, 125, 157, 41, 83, 167, 237, 115},
			shouldFail: true,
		},
		"larger than modulus 3": {
			bytes:      []byte{1, 0, 0, 0, 255, 255, 255, 255, 254, 91, 254, 255, 2, 164, 189, 83, 5, 216, 161, 9, 8, 216, 57, 51, 72, 125, 157, 41, 83, 167, 237, 116},
			shouldFail: true,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			actual, err := scalarFromBytes(tc.bytes, false)
			if tc.shouldFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.expected != nil {
					require.Equal(t, tc.expected, actual)
				}
			}
		})
	}
}

func TestKeccak256(t *testing.T) {
	input := []byte("hello")
	data := Keccak256(input)

	expected := "1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8"
	actual := hex.EncodeToString(data.BytesBE())

	require.Equal(t, expected, actual)
}

func TestBlsMultiExp(t *testing.T) {
	const (
		g1Hex = "97f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb"
		g2Hex = "93e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e" +
			"024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb8"
	)
	g1, err := hex.DecodeString(g1Hex)
	require.NoError(t, err)

	g2, err := hex.DecodeString(g2Hex)
	require.NoError(t, err)

	crypto := newCrypto()

	t.Run("multiExpG1", func(t *testing.T) {
		var g1Point bls12381.G1Affine
		_, err = g1Point.SetBytes(g1)
		require.NoError(t, err)
		one := make([]byte, fr.Bytes)
		bigint.ToPreallocatedBytes(new(big.Int).SetUint64(1), one)
		two := make([]byte, fr.Bytes)
		bigint.ToPreallocatedBytes(new(big.Int).SetUint64(2), two)

		pairs := stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: &g1Point}),
				stackitem.NewByteArray(one),
			}),
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: &g1Point}),
				stackitem.NewByteArray(two),
			}),
		})
		actual, ok := crypto.bls12381MultiExp(nil, []stackitem.Item{pairs}).(*stackitem.Interop).Value().(blsPoint)
		require.True(t, ok)
		expected, err := blsPointMul(blsPoint{point: &g1Point}, new(big.Int).SetUint64(3))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("multiExpG2", func(t *testing.T) {
		var g2Point bls12381.G2Affine
		_, err = g2Point.SetBytes(g2)
		require.NoError(t, err)
		five := make([]byte, fr.Bytes)
		bigint.ToPreallocatedBytes(new(big.Int).SetUint64(5), five)

		pairs := stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: &g2Point}),
				stackitem.NewByteArray(five),
			}),
		})
		actual, ok := crypto.bls12381MultiExp(nil, []stackitem.Item{pairs}).(*stackitem.Interop).Value().(blsPoint)
		require.True(t, ok)
		expected, err := blsPointMul(blsPoint{point: &g2Point}, new(big.Int).SetUint64(5))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("reduces scalar", func(t *testing.T) {
		var g1Point bls12381.G1Affine
		_, err = g1Point.SetBytes(g1)
		require.NoError(t, err)
		twoPlusR := make([]byte, fr.Bytes)
		bigint.ToPreallocatedBytes(new(big.Int).Add(fr.Modulus(), big.NewInt(2)), twoPlusR)

		pairs := stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: &g1Point}),
				stackitem.NewByteArray(twoPlusR),
			}),
		})
		actual, ok := crypto.bls12381MultiExp(nil, []stackitem.Item{pairs}).(*stackitem.Interop).Value().(blsPoint)
		require.True(t, ok)
		expected, err := blsPointMul(blsPoint{point: &g1Point}, new(big.Int).SetUint64(2))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("empty pairs", func(t *testing.T) {
		require.PanicsWithValue(t, "BLS12-381 multi exponent requires at least one pair", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{})})
		})
	})
	t.Run("too many pairs", func(t *testing.T) {
		require.PanicsWithValue(t, fmt.Sprintf("BLS12-381 multi exponent supports at most %d pairs", Bls12381MultiExpMaxPairs), func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray(make([]stackitem.Item, Bls12381MultiExpMaxPairs+1))})
		})
	})
	t.Run("invalid pair type", func(t *testing.T) {
		require.PanicsWithValue(t, "BLS12-381 multi exponent pair must be Array or Struct", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewMap(),
			})})
		})
	})
	t.Run("invalid pair length", func(t *testing.T) {
		require.PanicsWithValue(t, "BLS12-381 multi exponent pair must contain point and scalar", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewArray(nil),
			})})
		})
	})
}
