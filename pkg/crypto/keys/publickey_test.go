package keys

import (
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"math/rand"
	"sort"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeInfinity(t *testing.T) {
	key := &PublicKey{}
	b, err := testserdes.EncodeBinary(key)
	require.NoError(t, err)
	require.Equal(t, 1, len(b))

	keyDecode := &PublicKey{}
	require.NoError(t, keyDecode.DecodeBytes(b))
	require.Equal(t, []byte{0x00}, keyDecode.Bytes())
}

func TestEncodeDecodePublicKey(t *testing.T) {
	for i := 0; i < 4; i++ {
		k, err := NewPrivateKey()
		require.NoError(t, err)
		p := k.PublicKey()
		testserdes.EncodeDecodeBinary(t, p, new(PublicKey))
	}

	errCases := [][]byte{{}, {0x02}, {0x04}}

	for _, tc := range errCases {
		require.Error(t, testserdes.DecodeBinary(tc, new(PublicKey)))
	}
}

func TestDecodeFromString(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	pubKey, err := NewPublicKeyFromString(str)
	require.NoError(t, err)
	require.Equal(t, str, hex.EncodeToString(pubKey.Bytes()))

	_, err = NewPublicKeyFromString(str[2:])
	require.Error(t, err)

	str = "zzb209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	_, err = NewPublicKeyFromString(str)
	require.Error(t, err)
}

func TestDecodeFromStringBadCompressed(t *testing.T) {
	str := "02ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	_, err := NewPublicKeyFromString(str)
	require.Error(t, err)
}

func TestDecodeFromStringBadXMoreThanP(t *testing.T) {
	str := "02ffffffff00000001000000000000000000000001ffffffffffffffffffffffff"
	_, err := NewPublicKeyFromString(str)
	require.Error(t, err)
}

func TestDecodeFromStringNotOnCurve(t *testing.T) {
	str := "04ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	_, err := NewPublicKeyFromString(str)
	require.Error(t, err)
}

func TestDecodeFromStringUncompressed(t *testing.T) {
	str := "046b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c2964fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5"
	_, err := NewPublicKeyFromString(str)
	require.NoError(t, err)
}

func TestPubkeyToAddress(t *testing.T) {
	pubKey, err := NewPublicKeyFromString("031ee4e73a17d8f76dc02532e2620bcb12425b33c0c9f9694cc2caa8226b68cad4")
	require.NoError(t, err)
	actual := pubKey.Address()
	expected := "AUpGsNCHzSimeMRVPQfhwrVdiUp8Q2N2Qx"
	require.Equal(t, expected, actual)
}

func TestDecodeBytes(t *testing.T) {
	pubKey := getPubKey(t)
	decodedPubKey := &PublicKey{}
	err := decodedPubKey.DecodeBytes(pubKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, pubKey, decodedPubKey)
}

func TestDecodeBytesBadInfinity(t *testing.T) {
	decodedPubKey := &PublicKey{}
	err := decodedPubKey.DecodeBytes([]byte{0, 0, 0})
	require.Error(t, err)
}

func TestSort(t *testing.T) {
	pubs1 := make(PublicKeys, 10)
	for i := range pubs1 {
		priv, err := NewPrivateKey()
		require.NoError(t, err)
		pubs1[i] = priv.PublicKey()
	}

	pubs2 := make(PublicKeys, len(pubs1))
	copy(pubs2, pubs1)

	sort.Sort(pubs1)

	rand.Shuffle(len(pubs2), func(i, j int) {
		pubs2[i], pubs2[j] = pubs2[j], pubs2[i]
	})
	sort.Sort(pubs2)

	// Check that sort on the same set of values produce the same result.
	require.Equal(t, pubs1, pubs2)
}

func TestContains(t *testing.T) {
	pubKey := getPubKey(t)
	pubKeys := &PublicKeys{getPubKey(t)}
	pubKeys.Contains(pubKey)
	require.True(t, pubKeys.Contains(pubKey))
}

func TestUnique(t *testing.T) {
	pubKeys := &PublicKeys{getPubKey(t), getPubKey(t)}
	unique := pubKeys.Unique()
	require.Equal(t, 1, unique.Len())
}

func getPubKey(t *testing.T) *PublicKey {
	pubKey, err := NewPublicKeyFromString("031ee4e73a17d8f76dc02532e2620bcb12425b33c0c9f9694cc2caa8226b68cad4")
	require.NoError(t, err)
	return pubKey
}

func TestMarshallJSON(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	pubKey, err := NewPublicKeyFromString(str)
	require.NoError(t, err)

	bytes, err := json.Marshal(&pubKey)
	require.NoError(t, err)
	require.Equal(t, []byte(`"`+str+`"`), bytes)
}

func TestUnmarshallJSON(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	expected, err := NewPublicKeyFromString(str)
	require.NoError(t, err)

	actual := &PublicKey{}
	err = json.Unmarshal([]byte(`"`+str+`"`), actual)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestUnmarshallJSONBadCompresed(t *testing.T) {
	str := `"02ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"`
	actual := &PublicKey{}
	err := json.Unmarshal([]byte(str), actual)
	require.Error(t, err)
}

func TestUnmarshallJSONNotAHex(t *testing.T) {
	str := `"04Tb17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c2964fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5"`
	actual := &PublicKey{}
	err := json.Unmarshal([]byte(str), actual)
	require.Error(t, err)
}

func TestUnmarshallJSONBadFormat(t *testing.T) {
	str := "046b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c2964fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5"
	actual := &PublicKey{}
	err := json.Unmarshal([]byte(str), actual)
	require.Error(t, err)
}

func TestRecoverSecp256r1(t *testing.T) {
	privateKey, err := NewPrivateKey()
	require.NoError(t, err)
	message := []byte{72, 101, 108, 108, 111, 87, 111, 114, 108, 100}
	messageHash := hash.Sha256(message).BytesBE()
	signature := privateKey.Sign(message)
	r := new(big.Int).SetBytes(signature[0:32])
	s := new(big.Int).SetBytes(signature[32:64])
	require.True(t, privateKey.PublicKey().Verify(signature, messageHash))
	// To test this properly, we should provide correct isEven flag. This flag denotes which one of
	// the two recovered R points in decodeCompressedY method should be chosen. Let's suppose that we
	// don't know which of them suites, so to test KeyRecover we should check both and only
	// one of them gives us the correct public key.
	recoveredKeyFalse, err := KeyRecover(elliptic.P256(), r, s, messageHash, false)
	require.NoError(t, err)
	recoveredKeyTrue, err := KeyRecover(elliptic.P256(), r, s, messageHash, true)
	require.NoError(t, err)
	require.True(t, privateKey.PublicKey().Equal(&recoveredKeyFalse) != privateKey.PublicKey().Equal(&recoveredKeyTrue))
}

func TestRecoverSecp256r1Static(t *testing.T) {
	// These data were taken from the reference KeyRecoverTest: https://github.com/neo-project/neo/blob/neox-2.x/neo.UnitTests/UT_ECDsa.cs#L22
	// To update this test, run the reference KeyRecover(ECCurve.Secp256r1) testcase and fetch the following data from it:
	// privateKey -> b
	// message -> messageHash
	// signatures[0] -> r
	// signatures[1] -> s
	// v -> isEven
	// Note, that C# BigInteger has different byte order from that used in Go.
	b := []byte{123, 245, 126, 56, 3, 123, 197, 199, 26, 31, 212, 186, 120, 195, 168, 153, 57, 108, 234, 49, 107, 203, 44, 207, 185, 212, 187, 129, 74, 43, 225, 69}
	privateKey, err := NewPrivateKeyFromBytes(b)
	require.NoError(t, err)
	messageHash := []byte{72, 101, 108, 108, 111, 87, 111, 114, 108, 100}
	r := new(big.Int).SetBytes([]byte{1, 85, 226, 63, 133, 113, 217, 188, 249, 22, 213, 203, 225, 199, 32, 131, 118, 23, 28, 101, 139, 211, 13, 111, 242, 158, 193, 227, 196, 106, 3, 4})
	s := new(big.Int).SetBytes([]byte{65, 174, 206, 164, 81, 34, 76, 104, 5, 49, 51, 20, 221, 183, 157, 199, 199, 47, 78, 137, 172, 99, 212, 110, 129, 72, 236, 59, 250, 81, 200, 13})
	// Just ensure it's a valid signature.
	require.True(t, privateKey.PublicKey().Verify(append(r.Bytes(), s.Bytes()...), messageHash))
	recoveredKey, err := KeyRecover(elliptic.P256(), r, s, messageHash, false)
	require.NoError(t, err)
	require.True(t, privateKey.PublicKey().Equal(&recoveredKey))
}

func TestRecoverSecp256k1(t *testing.T) {
	privateKey, err := btcec.NewPrivateKey(btcec.S256())
	message := []byte{72, 101, 108, 108, 111, 87, 111, 114, 108, 100}
	signature, err := privateKey.Sign(message)
	require.NoError(t, err)
	require.True(t, signature.Verify(message, privateKey.PubKey()))
	// To test this properly, we should provide correct isEven flag. This flag denotes which one of
	// the two recovered R points in decodeCompressedY method should be chosen. Let's suppose that we
	// don't know which of them suites, so to test KeyRecover we should check both and only
	// one of them gives us the correct public key.
	recoveredKeyFalse, err := KeyRecover(btcec.S256(), signature.R, signature.S, message, false)
	require.NoError(t, err)
	recoveredKeyTrue, err := KeyRecover(btcec.S256(), signature.R, signature.S, message, true)
	require.NoError(t, err)
	require.True(t, (privateKey.PubKey().X.Cmp(recoveredKeyFalse.X) == 0 &&
		privateKey.PubKey().Y.Cmp(recoveredKeyFalse.Y) == 0) !=
		(privateKey.PubKey().X.Cmp(recoveredKeyTrue.X) == 0 &&
			privateKey.PubKey().Y.Cmp(recoveredKeyTrue.Y) == 0))
}

func TestRecoverSecp256k1Static(t *testing.T) {
	// These data were taken from the reference testcase: https://github.com/neo-project/neo/blob/neox-2.x/neo.UnitTests/UT_ECDsa.cs#L22
	// To update this test, run the reference KeyRecover(ECCurve.Secp256k1) testcase and fetch the following data from it:
	// privateKey -> b
	// message -> messageHash
	// signatures[0] -> r
	// signatures[1] -> s
	// v -> isEven
	// Note, that C# BigInteger has different byte order from that used in Go.
	b := []byte{156, 3, 247, 58, 246, 250, 236, 27, 118, 60, 180, 177, 18, 92, 204, 206, 144, 245, 148, 141, 86, 212, 151, 181, 15, 113, 172, 180, 177, 228, 100, 32}
	_, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), b)
	messageHash := []byte{72, 101, 108, 108, 111, 87, 111, 114, 108, 100}
	r := new(big.Int).SetBytes([]byte{88, 169, 242, 111, 210, 184, 180, 46, 67, 108, 176, 77, 57, 250, 58, 36, 110, 81, 225, 65, 90, 47, 215, 91, 27, 227, 57, 6, 9, 228, 100, 50})
	s := new(big.Int).SetBytes([]byte{86, 150, 81, 190, 17, 181, 212, 241, 184, 36, 136, 116, 232, 207, 46, 45, 149, 167, 15, 98, 113, 137, 66, 98, 214, 165, 38, 232, 98, 96, 79, 197})
	signature := btcec.Signature{
		R: r,
		S: s,
	}
	// Just ensure it's a valid signature.
	require.True(t, signature.Verify(messageHash, publicKey))
	recoveredKey, err := KeyRecover(btcec.S256(), r, s, messageHash, false)
	require.NoError(t, err)
	require.True(t, new(big.Int).SetBytes([]byte{112, 186, 29, 131, 169, 21, 212, 95, 81, 172, 201, 145, 168, 108, 129, 90, 6, 111, 80, 39, 136, 157, 15, 181, 98, 108, 133, 108, 144, 80, 23, 225}).Cmp(recoveredKey.X) == 0)
	require.True(t, new(big.Int).SetBytes([]byte{187, 102, 202, 42, 152, 133, 222, 55, 137, 228, 154, 80, 182, 35, 133, 14, 55, 165, 36, 64, 178, 55, 13, 112, 224, 143, 66, 143, 208, 18, 2, 211}).Cmp(recoveredKey.Y) == 0)
}
