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

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
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
				_ = c.verifyWithECDsaV1(ic, argsArr)
			})
		} else {
			require.NotPanics(t, func() {
				actual = c.verifyWithECDsaV1(ic, argsArr)
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
				_ = c.verifyWithEd25519V0(ic, argsArr)
			})
		} else {
			require.NotPanics(t, func() {
				actual = c.verifyWithEd25519V0(ic, argsArr)
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

func TestED25519Spec(t *testing.T) {
	t.Skip()
	var spec = []struct {
		message   string
		pubKey    string
		signature string
	}{{message: "8c93255d71dcab10e8f379c26200f3c7bd5f09d9bc3068d3ef4edeb4853022b6", pubKey: "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac03fa", signature: "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a0000000000000000000000000000000000000000000000000000000000000000"}, {message: "9bd9f44f4dcc75bd531b56b2cd280b0bb38fc1cd6d1230e14861d861de092e79", pubKey: "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac03fa", signature: "f7badec5b8abeaf699583992219b7b223f1df3fbbea919844e3f7c554a43dd43a5bb704786be79fc476f91d3f3f89b03984d8068dcf1bb7dfc6637b45450ac04"}, {message: "aebf3f2601a0c8c5d39cc7d8911642f740b78168218da8471772b35f9d35b9ab", pubKey: "f7badec5b8abeaf699583992219b7b223f1df3fbbea919844e3f7c554a43dd43", signature: "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac03fa8c4bd45aecaca5b24fb97bc10ac27ac8751a7dfe1baff8b953ec9f5833ca260e"}, {message: "9bd9f44f4dcc75bd531b56b2cd280b0bb38fc1cd6d1230e14861d861de092e79", pubKey: "cdb267ce40c5cd45306fa5d2f29731459387dbf9eb933b7bd5aed9a765b88d4d", signature: "9046a64750444938de19f227bb80485e92b83fdb4b6506c160484c016cc1852f87909e14428a7a1d62e9f22f3d3ad7802db02eb2e688b6c52fcd6648a98bd009"}, {message: "e47d62c63f830dc7a6851a0b1f33ae4bb2f507fb6cffec4011eaccd55b53f56c", pubKey: "cdb267ce40c5cd45306fa5d2f29731459387dbf9eb933b7bd5aed9a765b88d4d", signature: "160a1cb0dc9c0258cd0a7d23e94d8fa878bcb1925f2c64246b2dee1796bed5125ec6bc982a269b723e0668e540911a9a6a58921d6925e434ab10aa7940551a09"}, {message: "e47d62c63f830dc7a6851a0b1f33ae4bb2f507fb6cffec4011eaccd55b53f56c", pubKey: "cdb267ce40c5cd45306fa5d2f29731459387dbf9eb933b7bd5aed9a765b88d4d", signature: "21122a84e0b5fca4052f5b1235c80a537878b38f3142356b2c2384ebad4668b7e40bc836dac0f71076f9abe3a53f9c03c1ceeeddb658d0030494ace586687405"}, {message: "85e241a07d148b41e47d62c63f830dc7a6851a0b1f33ae4bb2f507fb6cffec40", pubKey: "442aad9f089ad9e14647b1ef9099a1ff4798d78589e66f28eca69c11f582a623", signature: "e96f66be976d82e60150baecff9906684aebb1ef181f67a7189ac78ea23b6c0e547f7690a0e2ddcd04d87dbc3490dc19b3b3052f7ff0538cb68afb369ba3a514"}, {message: "85e241a07d148b41e47d62c63f830dc7a6851a0b1f33ae4bb2f507fb6cffec40", pubKey: "442aad9f089ad9e14647b1ef9099a1ff4798d78589e66f28eca69c11f582a623", signature: "8ce5b96c8f26d0ab6c47958c9e68b937104cd36e13c33566acd2fe8d38aa19427e71f98a473474f2f13f06f97c20d58cc3f54b8bd0d272f42b695dd7e89a8c22"}, {message: "9bedc267423725d473888631ebf45988bad3db83851ee85c85e241a07d148b41", pubKey: "f7badec5b8abeaf699583992219b7b223f1df3fbbea919844e3f7c554a43dd43", signature: "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff03be9678ac102edcd92b0210bb34d7428d12ffc5df5f37e359941266a4e35f0f"}, {message: "9bedc267423725d473888631ebf45988bad3db83851ee85c85e241a07d148b41", pubKey: "f7badec5b8abeaf699583992219b7b223f1df3fbbea919844e3f7c554a43dd43", signature: "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffca8c5b64cd208982aa38d4936621a4775aa233aa0505711d8fdcfdaa943d4908"}, {message: "e96b7021eb39c1a163b6da4e3093dcd3f21387da4cc4572be588fafae23c155b", pubKey: "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", signature: "a9d55260f765261eb9b84e106f665e00b867287a761990d7135963ee0a7d59dca5bb704786be79fc476f91d3f3f89b03984d8068dcf1bb7dfc6637b45450ac04"}, {message: "39a591f5321bbe07fd5a23dc2f39d025d74526615746727ceefd6e82ae65c06f", pubKey: "ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", signature: "a9d55260f765261eb9b84e106f665e00b867287a761990d7135963ee0a7d59dca5bb704786be79fc476f91d3f3f89b03984d8068dcf1bb7dfc6637b45450ac04"}}

	var (
		good = []string{"false", "throws", "throws", "throws", "throws", "throws", "throws", "throws", "throws", "throws", "false", "false"}
		res  []string
	)

	for _, vector := range spec {
		msg, err := hex.DecodeString(vector.message)
		require.NoError(t, err)
		pk, err := hex.DecodeString(vector.pubKey)
		require.NoError(t, err)
		sig, err := hex.DecodeString(vector.signature)
		require.NoError(t, err)

		res = append(res, fmt.Sprintf("%v", ed25519.Verify(pk, msg, sig)))
	}
	require.Equal(t, good, res)
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
