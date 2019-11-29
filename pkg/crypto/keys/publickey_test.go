package keys

import (
	"encoding/hex"
	"math/rand"
	"sort"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeInfinity(t *testing.T) {
	key := &PublicKey{}
	buf := io.NewBufBinWriter()
	key.EncodeBinary(buf.BinWriter)
	require.NoError(t, buf.Err)
	b := buf.Bytes()
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
		buf := io.NewBufBinWriter()
		p.EncodeBinary(buf.BinWriter)
		require.NoError(t, buf.Err)
		b := buf.Bytes()

		pDecode := &PublicKey{}
		require.NoError(t, pDecode.DecodeBytes(b))
		require.Equal(t, p.X, pDecode.X)
	}

	errCases := [][]byte{{}, {0x02}, {0x04}}

	for _, tc := range errCases {
		r := io.NewBinReaderFromBuf(tc)

		var pDecode PublicKey
		pDecode.DecodeBinary(r)
		require.Error(t, r.Err)
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
	require.Equal(t, pubKey,decodedPubKey)
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
