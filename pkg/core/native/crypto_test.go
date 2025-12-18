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

const (
	g1Hex                            = "97f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb"
	g2Hex                            = "93e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb8"
	ethG1MultiExpSingleInputHex      = "0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e10000000000000000000000000000000000000000000000000000000000000011"
	ethG1MultiExpSingleExpectedHex   = "000000000000000000000000000000001098f178f84fc753a76bb63709e9be91eec3ff5f7f3a5f4836f34fe8a1a6d6c5578d8fd820573cef3a01e2bfef3eaf3a000000000000000000000000000000000ea923110b733b531006075f796cc9368f2477fe26020f465468efbb380ce1f8eebaf5c770f31d320f9bd378dc758436"
	ethG1MultiExpMultipleInputHex    = "0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e10000000000000000000000000000000000000000000000000000000000000032000000000000000000000000000000000e12039459c60491672b6a6282355d8765ba6272387fb91a3e9604fa2a81450cf16b870bb446fc3a3e0a187fff6f89450000000000000000000000000000000018b6c1ed9f45d3cbc0b01b9d038dcecacbd702eb26469a0eb3905bd421461712f67f782b4735849644c1772c93fe3d09000000000000000000000000000000000000000000000000000000000000003300000000000000000000000000000000147b327c8a15b39634a426af70c062b50632a744eddd41b5a4686414ef4cd9746bb11d0a53c6c2ff21bbcf331e07ac9200000000000000000000000000000000078c2e9782fa5d9ab4e728684382717aa2b8fad61b5f5e7cf3baa0bc9465f57342bb7c6d7b232e70eebcdbf70f903a450000000000000000000000000000000000000000000000000000000000000034"
	ethG1MultiExpMultipleExpectedHex = "000000000000000000000000000000001339b4f51923efe38905f590ba2031a2e7154f0adb34a498dfde8fb0f1ccf6862ae5e3070967056385055a666f1b6fc70000000000000000000000000000000009fb423f7e7850ef9c4c11a119bb7161fe1d11ac5527051b29fe8f73ad4262c84c37b0f1b9f0e163a9682c22c7f98c80"
	ethG2MultiExpSingleInputHex      = "00000000000000000000000000000000024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb80000000000000000000000000000000013e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e000000000000000000000000000000000ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801000000000000000000000000000000000606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be0000000000000000000000000000000000000000000000000000000000000011"
	ethG2MultiExpSingleExpectedHex   = "000000000000000000000000000000000ef786ebdcda12e142a32f091307f2fedf52f6c36beb278b0007a03ad81bf9fee3710a04928e43e541d02c9be44722e8000000000000000000000000000000000d05ceb0be53d2624a796a7a033aec59d9463c18d672c451ec4f2e679daef882cab7d8dd88789065156a1340ca9d426500000000000000000000000000000000118ed350274bc45e63eaaa4b8ddf119b3bf38418b5b9748597edfc456d9bc3e864ec7283426e840fd29fa84e7d89c934000000000000000000000000000000001594b866a28946b6d444bf0481558812769ea3222f5dfc961ca33e78e0ea62ee8ba63fd1ece9cc3e315abfa96d536944"
)

var (
	g1, _                            = hex.DecodeString(g1Hex)
	g2, _                            = hex.DecodeString(g2Hex)
	ethG1MultiExpSingleInput, _      = hex.DecodeString(ethG1MultiExpSingleInputHex)
	ethG1MultiExpSingleExpected, _   = hex.DecodeString(ethG1MultiExpSingleExpectedHex)
	ethG1MultiExpMultipleInput, _    = hex.DecodeString(ethG1MultiExpMultipleInputHex)
	ethG1MultiExpMultipleExpected, _ = hex.DecodeString(ethG1MultiExpMultipleExpectedHex)
	ethG2MultiExpSingleInput, _      = hex.DecodeString(ethG2MultiExpSingleInputHex)
	ethG2MultiExpSingleExpected, _   = hex.DecodeString(ethG2MultiExpSingleExpectedHex)

	g1Point = func() *bls12381.G1Affine {
		a := new(bls12381.G1Affine)
		_, _ = a.SetBytes(g1)
		return a
	}()
	g2Point = func() *bls12381.G2Affine {
		a := new(bls12381.G2Affine)
		_, _ = a.SetBytes(g2)
		return a
	}()

	g1Uncompressed = g1Point.RawBytes()
	g2Uncompressed = g2Point.RawBytes()
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
		expected   *big.Int
		shouldFail bool
	}{
		"zero": {
			bytes:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expected: new(big.Int).SetUint64(0),
		},
		"one": {
			bytes:    []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expected: new(big.Int).SetUint64(1),
		},
		"R2": {
			bytes:    []byte{254, 255, 255, 255, 1, 0, 0, 0, 2, 72, 3, 0, 250, 183, 132, 88, 245, 79, 188, 236, 239, 79, 140, 153, 111, 5, 197, 172, 89, 177, 36, 24},
			expected: r2Ref.BigInt(new(big.Int)),
		},
		"negative": {
			bytes: []byte{0, 0, 0, 0, 255, 255, 255, 255, 254, 91, 254, 255, 2, 164, 189, 83, 5, 216, 161, 9, 8, 216, 57, 51, 72, 125, 157, 41, 83, 167, 237, 115},
		},
		"invalid length": {
			shouldFail: true,
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
					require.Zero(t, tc.expected.Cmp(actual))
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
	crypto := newCrypto()
	t.Run("multiExpG1", func(t *testing.T) {
		pairs := stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: g1Point}),
				stackitem.NewBigInteger(new(big.Int).SetUint64(1)),
			}),
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: g1Point}),
				stackitem.NewBigInteger(new(big.Int).SetUint64(2)),
			}),
		})
		actual, ok := crypto.bls12381MultiExp(nil, []stackitem.Item{pairs}).(*stackitem.Interop).Value().(blsPoint)
		require.True(t, ok)
		expected, err := blsPointMul(blsPoint{point: g1Point}, new(big.Int).SetUint64(3))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("multiExpG2", func(t *testing.T) {
		pairs := stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: g2Point}),
				stackitem.NewBigInteger(new(big.Int).SetUint64(5)),
			}),
		})
		actual, ok := crypto.bls12381MultiExp(nil, []stackitem.Item{pairs}).(*stackitem.Interop).Value().(blsPoint)
		require.True(t, ok)
		expected, err := blsPointMul(blsPoint{point: g2Point}, new(big.Int).SetUint64(5))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})

	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "bls12381 multi exponent requires at least one pair", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{})})
		})
		require.PanicsWithValue(t, fmt.Sprintf("bls12381 multi exponent supports at most %d pairs", bls12381MultiExpMaxPairs), func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray(make([]stackitem.Item, bls12381MultiExpMaxPairs+1))})
		})
		require.PanicsWithValue(t, "bls12381 multi exponent pair must be Array or Struct", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewMap(),
			})})
		})
		require.PanicsWithValue(t, "bls12381 multi exponent pair must contain point and scalar", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewArray(nil),
			})})
		})
		require.PanicsWithValue(t, "bls12381 multi exponent requires interop points", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.Null{},
					nil,
				}),
			})})
		})
		require.PanicsWithValue(t, "bls12381 multi exponent interop must contain blsPoint", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewInterop(nil),
					nil,
				}),
			})})
		})
		require.Panics(t, func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewInterop(blsPoint{g1Point}),
					stackitem.NewBigInteger(new(big.Int).SetUint64(1)),
				}),
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewInterop(blsPoint{g2Point}),
					stackitem.NewBigInteger(new(big.Int).SetUint64(1)),
				}),
			})})
		})
		require.PanicsWithValue(t, "invalid bls12381 point type", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewInterop(blsPoint{}),
					nil,
				}),
			})})
		})
		require.Panics(t, func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewInterop(blsPoint{g1Point}),
					stackitem.Null{},
				}),
			})})
		})
		require.PanicsWithValue(t, "bls12381 multi exponent requires at least one valid pair", func() {
			crypto.bls12381MultiExp(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewInterop(blsPoint{g1Point}),
					stackitem.NewBigInteger(new(big.Int)),
				}),
			})})
		})
	})
}

func TestBlsPairingList(t *testing.T) {
	crypto := newCrypto()
	t.Run("success case", func(t *testing.T) {
		actual := crypto.bls12381PairingList(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
			stackitem.NewInterop(blsPoint{point: g1Point}),
			stackitem.NewInterop(blsPoint{point: g2Point}),
			stackitem.NewInterop(blsPoint{point: new(bls12381.G1Affine).Neg(g1Point)}),
			stackitem.NewInterop(blsPoint{point: g2Point}),
		})}).Value().(bool)
		require.True(t, actual)
	})
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "bls12381 pairing requires at least one pair", func() {
			crypto.bls12381PairingList(nil, []stackitem.Item{stackitem.NewArray(nil)})
		})
		require.PanicsWithError(t, fmt.Sprintf("bls12381 pairing supports at most %d pairs", bls12381PairingMaxPairs), func() {
			crypto.bls12381PairingList(nil, []stackitem.Item{stackitem.NewArray(make([]stackitem.Item, 2*(bls12381PairingMaxPairs+1)))})
		})
		require.PanicsWithValue(t, "bls12381 pairing requires an even number of elements", func() {
			crypto.bls12381PairingList(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{nil})})
		})
		require.PanicsWithValue(t, "bls12381 pairing requires an even number of elements", func() {
			crypto.bls12381PairingList(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{nil})})
		})
		require.PanicsWithValue(t, "bls12381 pairing requires interop points", func() {
			crypto.bls12381PairingList(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{stackitem.Null{}, nil})})
		})
		require.PanicsWithValue(t, "interop must contain bls12381 point", func() {
			crypto.bls12381PairingList(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(nil),
				stackitem.NewInterop(nil),
			})})
		})
		require.Panics(t, func() {
			crypto.bls12381PairingList(nil, []stackitem.Item{stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: g1Point}),
				stackitem.NewInterop(blsPoint{point: g1Point}),
			})})
		})
	})
}

func TestBlsSerializeEth(t *testing.T) {
	crypto := newCrypto()
	t.Run("success case g1", func(t *testing.T) {
		for _, p := range []*bls12381.G1Affine{g1Point, {}} {
			bi := crypto.bls12381SerializeEth(nil, []stackitem.Item{stackitem.NewInterop(blsPoint{p})})
			actual, ok := bi.Value().([]byte)
			require.True(t, ok)
			uncompressed := p.RawBytes()
			uncompressed[0] &= 0b00011111
			require.Equal(t, addPadding(uncompressed[:]), actual)
		}
	})
	t.Run("errors", func(t *testing.T) {
		require.Panics(t, func() {
			crypto.bls12381SerializeEth(nil, []stackitem.Item{stackitem.NewInterop(nil)})
		})
		require.PanicsWithError(t, "invalid bls12381 point type", func() {
			crypto.bls12381SerializeEth(nil, []stackitem.Item{stackitem.NewInterop(blsPoint{})})
		})
	})
}

func TestBlsDeserializeEth(t *testing.T) {
	crypto := newCrypto()
	t.Run("success case g2", func(t *testing.T) {
		for _, expected := range []*bls12381.G2Affine{g2Point, {}} {
			uncompressed := expected.RawBytes()
			uncompressed[0] &= 0b00011111
			input := g2ToEthereum(uncompressed[:])
			pi := crypto.bls12381DeserializeEth(nil, []stackitem.Item{stackitem.NewByteArray(input)})
			actual, ok := pi.Value().(blsPoint).point.(*bls12381.G2Affine)
			require.True(t, ok)
			require.Equal(t, expected, actual)
		}
	})
	t.Run("errors", func(t *testing.T) {
		require.Panics(t, func() {
			crypto.bls12381DeserializeEth(nil, []stackitem.Item{stackitem.Null{}})
		})
		require.Panics(t, func() {
			crypto.bls12381DeserializeEth(nil, []stackitem.Item{stackitem.NewByteArray([]byte{})})
		})
	})
}

func TestBlsSerializeList(t *testing.T) {
	crypto := newCrypto()
	t.Run("point and scalar pairs", func(t *testing.T) {
		pairs := stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{g1Point}),
				stackitem.NewBigInteger(new(big.Int).SetUint64(1)),
			}),
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{g1Point}),
				stackitem.NewBigInteger(new(big.Int).SetUint64(3)),
			}),
		})
		actual := crypto.bls12381SerializeList(nil, []stackitem.Item{pairs}).Value().([]byte)
		g1Bytes := g1Point.Bytes()
		one := make([]byte, fr.Bytes)
		copy(one, bigint.ToPreallocatedBytes(new(big.Int).SetUint64(1), nil))
		three := make([]byte, fr.Bytes)
		copy(three, bigint.ToPreallocatedBytes(new(big.Int).SetUint64(3), nil))
		expected := g1Bytes[:]
		expected = append(expected, one...)
		expected = append(expected, g1Bytes[:]...)
		expected = append(expected, three...)
		require.Equal(t, expected, actual)
	})
	t.Run("points", func(t *testing.T) {
		points := stackitem.NewArray([]stackitem.Item{
			stackitem.NewInterop(blsPoint{g1Point}),
			stackitem.NewInterop(blsPoint{g2Point}),
		})
		actual := crypto.bls12381SerializeList(nil, []stackitem.Item{points}).Value().([]byte)
		g1Bytes := g1Point.Bytes()
		g2Bytes := g2Point.Bytes()
		expected := g1Bytes[:]
		expected = append(expected, g2Bytes[:]...)
		require.Equal(t, expected, actual)
	})
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "at least one element is required", func() {
			crypto.bls12381SerializeList(nil, []stackitem.Item{
				stackitem.NewArray(nil),
			})
		})
		require.PanicsWithValue(t, "not a bls12381 point", func() {
			crypto.bls12381SerializeList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewInterop(nil),
				}),
			})
		})
		require.PanicsWithValue(t, "pair must be Array or Struct", func() {
			crypto.bls12381SerializeList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.Null{},
				}),
			})
		})
		require.PanicsWithValue(t, "pair must contain point and scalar", func() {
			crypto.bls12381SerializeList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray(nil),
				}),
			})
		})
		require.PanicsWithValue(t, "scalar must be bigint", func() {
			crypto.bls12381SerializeList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray([]stackitem.Item{
						nil,
						stackitem.Null{},
					}),
				}),
			})
		})
		require.PanicsWithValue(t, "not a bls12381 point", func() {
			crypto.bls12381SerializeList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray([]stackitem.Item{
						stackitem.NewInterop(nil),
						stackitem.NewBigInteger(big.NewInt(0)),
					}),
				}),
			})
		})
	})
}

func TestBlsSerializeEthList(t *testing.T) {
	crypto := newCrypto()
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "at least one element is required", func() {
			crypto.bls12381SerializeEthList(nil, []stackitem.Item{
				stackitem.NewArray(nil),
			})
		})
		require.PanicsWithValue(t, "not a bls12381 point", func() {
			crypto.bls12381SerializeEthList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewInterop(nil),
				}),
			})
		})
		require.PanicsWithValue(t, "pair must be Array or Struct", func() {
			crypto.bls12381SerializeEthList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.Null{},
				}),
			})
		})
		require.PanicsWithValue(t, "pair must contain point and scalar", func() {
			crypto.bls12381SerializeEthList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray(nil),
				}),
			})
		})
		require.PanicsWithValue(t, "scalar must be bigint", func() {
			crypto.bls12381SerializeEthList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray([]stackitem.Item{
						nil,
						stackitem.Null{},
					}),
				}),
			})
		})
		require.PanicsWithValue(t, "not a bls12381 point", func() {
			crypto.bls12381SerializeEthList(nil, []stackitem.Item{
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray([]stackitem.Item{
						stackitem.NewInterop(nil),
						stackitem.NewBigInteger(big.NewInt(0)),
					}),
				}),
			})
		})
	})
}

func TestDeserializeList(t *testing.T) {
	crypto := newCrypto()
	t.Run("success case", func(t *testing.T) {
		input := append(g1, g2...)
		expected := []stackitem.Item{
			stackitem.NewInterop(blsPoint{g1Point}),
			stackitem.NewInterop(blsPoint{g2Point}),
		}
		actual := crypto.bls12381DeserializeList(nil, []stackitem.Item{stackitem.NewByteArray(input)}).Value().([]stackitem.Item)
		require.Equal(t, expected, actual)
	})
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "deserializer requires at least one pair", func() {
			crypto.bls12381DeserializeList(nil, []stackitem.Item{
				stackitem.NewByteArray(nil),
			})
		})
		require.Panics(t, func() {
			crypto.bls12381DeserializeList(nil, []stackitem.Item{
				stackitem.NewByteArray([]byte{42}),
			})
		})
		require.Panics(t, func() {
			crypto.bls12381DeserializeList(nil, []stackitem.Item{
				stackitem.NewByteArray(make([]byte, bls12381.SizeOfG1AffineCompressed+bls12381.SizeOfG2AffineCompressed)),
			})
		})
	})
}

func TestDeserializeEthList(t *testing.T) {
	crypto := newCrypto()
	t.Run("eth compat", func(t *testing.T) {
		input := append(addPadding(g1Uncompressed[:]), g2ToEthereum(g2Uncompressed[:])...)
		points := crypto.bls12381DeserializeEthList(nil, []stackitem.Item{stackitem.NewByteArray(input)})
		actual := crypto.bls12381PairingList(nil, []stackitem.Item{points}).Value().(bool)
		expected, err := blsPointPairing(blsPoint{g1Point}, blsPoint{g2Point})
		require.NoError(t, err)
		require.Equal(t, expected.point.(*bls12381.GT).IsOne(), actual)
	})
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "deserializer requires at least one pair", func() {
			crypto.bls12381DeserializeEthList(nil, []stackitem.Item{
				stackitem.NewByteArray(nil),
			})
		})
		require.Panics(t, func() {
			crypto.bls12381DeserializeEthList(nil, []stackitem.Item{
				stackitem.NewByteArray([]byte{42}),
			})
		})
		require.Panics(t, func() {
			bytes := make([]byte, bls12G1EncodedLength+bls12G2EncodedLength)
			bytes[16] = 0b11100000
			crypto.bls12381DeserializeEthList(nil, []stackitem.Item{
				stackitem.NewByteArray(bytes),
			})
		})
	})
}

func TestDeserializeG1ScalarPairs(t *testing.T) {
	crypto := newCrypto()
	t.Run("success case g1", func(t *testing.T) {
		one := make([]byte, fr.Bytes)
		copy(one, bigint.ToPreallocatedBytes(new(big.Int).SetUint64(1), nil))
		input := append(g1, one...)
		input = append(input, g1...)
		three := make([]byte, fr.Bytes)
		copy(three, bigint.ToPreallocatedBytes(new(big.Int).SetUint64(3), nil))
		input = append(input, three...)
		expected := []stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: g1Point}),
				stackitem.NewBigInteger(new(big.Int).SetUint64(1)),
			}),
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewInterop(blsPoint{point: g1Point}),
				stackitem.NewBigInteger(new(big.Int).SetUint64(3)),
			}),
		}
		actual := crypto.bls12381DeserializeG1ScalarPairs(nil, []stackitem.Item{stackitem.NewByteArray(input)}).Value().([]stackitem.Item)
		require.Equal(t, expected, actual)
	})
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "deserializer requires at least one pair", func() {
			crypto.bls12381DeserializeG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(nil),
			})
		})
		require.Panics(t, func() {
			crypto.bls12381DeserializeG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray([]byte{42}),
			})
		})
		require.Panics(t, func() {
			bytes := make([]byte, bls12381.SizeOfG1AffineCompressed+fr.Bytes)
			bytes[0] = 0b11100000
			crypto.bls12381DeserializeG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(bytes),
			})
		})
		require.Panics(t, func() {
			mod := make([]byte, fr.Bytes)
			copy(mod, bigint.ToPreallocatedBytes(fr.Modulus(), nil))
			input := append(g1, mod...)
			crypto.bls12381DeserializeG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(input),
			})
		})
	})
}

func TestDeserializeG2ScalarPairs(t *testing.T) {
	crypto := newCrypto()
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "deserializer requires at least one pair", func() {
			crypto.bls12381DeserializeG2ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(nil),
			})
		})
		require.Panics(t, func() {
			crypto.bls12381DeserializeG2ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray([]byte{42}),
			})
		})
		require.Panics(t, func() {
			bytes := make([]byte, bls12381.SizeOfG2AffineCompressed+fr.Bytes)
			bytes[0] = 0b11100000
			crypto.bls12381DeserializeG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(bytes),
			})
		})
		require.Panics(t, func() {
			mod := make([]byte, fr.Bytes)
			copy(mod, bigint.ToPreallocatedBytes(fr.Modulus(), nil))
			input := append(g1, mod...)
			crypto.bls12381DeserializeG2ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(input),
			})
		})
	})
}

func TestDeserializeEthG1ScalarPairs(t *testing.T) {
	crypto := newCrypto()
	for _, params := range []struct{ input, expected []byte }{
		{ethG1MultiExpSingleInput, ethG1MultiExpSingleExpected},
		{ethG1MultiExpMultipleInput, ethG1MultiExpMultipleExpected},
	} {
		t.Run("eth compat", func(t *testing.T) {
			pairs := crypto.bls12381DeserializeEthG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(params.input),
			})
			res := crypto.bls12381MultiExp(nil, []stackitem.Item{pairs})
			bytes := crypto.bls12381SerializeEth(nil, []stackitem.Item{res})
			require.Equal(t, params.expected, bytes.Value().([]byte))
		})
	}
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "deserializer requires at least one pair", func() {
			crypto.bls12381DeserializeEthG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(nil),
			})
		})
		require.Panics(t, func() {
			crypto.bls12381DeserializeEthG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray([]byte{42}),
			})
		})
		require.Panics(t, func() {
			bytes := make([]byte, bls12G1EncodedLength+bls12ScalarLength)
			bytes[16] = 0b11100000
			crypto.bls12381DeserializeG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(bytes),
			})
		})
	})
}

func TestDeserializeEthG2ScalarPairs(t *testing.T) {
	crypto := newCrypto()
	t.Run("eth compat", func(t *testing.T) {
		pairs := crypto.bls12381DeserializeEthG2ScalarPairs(nil, []stackitem.Item{
			stackitem.NewByteArray(ethG2MultiExpSingleInput),
		})
		res := crypto.bls12381MultiExp(nil, []stackitem.Item{pairs})
		bytes := crypto.bls12381SerializeEth(nil, []stackitem.Item{res})
		require.Equal(t, ethG2MultiExpSingleExpected, bytes.Value().([]byte))
	})
	t.Run("errors", func(t *testing.T) {
		require.PanicsWithValue(t, "deserializer requires at least one pair", func() {
			crypto.bls12381DeserializeEthG2ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(nil),
			})
		})
		require.Panics(t, func() {
			crypto.bls12381DeserializeEthG2ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray([]byte{42}),
			})
		})
		require.Panics(t, func() {
			bytes := make([]byte, bls12G1EncodedLength+bls12ScalarLength)
			bytes[16] = 0b11100000
			crypto.bls12381DeserializeG1ScalarPairs(nil, []stackitem.Item{
				stackitem.NewByteArray(bytes),
			})
		})
	})
}
