package publickey

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/elliptic"
	"github.com/stretchr/testify/assert"
)

func TestDecodeFromString(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	pubKey, err := NewPublicKeyFromString(str)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, str, hex.EncodeToString(pubKey.Bytes()))
}

func TestEncodeDecodeInfinity(t *testing.T) {

	curve := elliptic.NewEllipticCurve(elliptic.Secp256r1)

	key := &PublicKey{curve, elliptic.Point{}}
	buf := new(bytes.Buffer)
	assert.Nil(t, key.EncodeBinary(buf))
	assert.Equal(t, 1, buf.Len())

	keyDecode := &PublicKey{}
	assert.Nil(t, keyDecode.DecodeBinary(buf))
	assert.Equal(t, []byte{0x00}, keyDecode.Bytes())
}

func TestEncodeDecodePublicKey(t *testing.T) {
	curve := elliptic.NewEllipticCurve(elliptic.Secp256r1)

	for i := 0; i < 4; i++ {
		p := &PublicKey{curve, randomECPoint()}
		buf := new(bytes.Buffer)
		assert.Nil(t, p.EncodeBinary(buf))

		pDecode := &PublicKey{curve, elliptic.Point{}}
		assert.Nil(t, pDecode.DecodeBinary(buf))
		assert.Equal(t, p.X, pDecode.X)
	}
}

func TestPubkeyToAddress(t *testing.T) {

	pubKey, err := NewPublicKeyFromString("031ee4e73a17d8f76dc02532e2620bcb12425b33c0c9f9694cc2caa8226b68cad4")
	if err != nil {
		t.Fatal(err)
	}

	actual := pubKey.ToAddress()
	expected := "AUpGsNCHzSimeMRVPQfhwrVdiUp8Q2N2Qx"
	assert.Equal(t, expected, actual)
}

func randomECPoint() elliptic.Point {
	curve := elliptic.NewEllipticCurve(elliptic.Secp256r1)
	b := make([]byte, curve.N.BitLen()/8+8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return elliptic.Point{}
	}

	d := new(big.Int).SetBytes(b)
	d.Mod(d, new(big.Int).Sub(curve.N, big.NewInt(1)))
	d.Add(d, big.NewInt(1))

	q := new(big.Int).SetBytes(d.Bytes())
	P1, P2 := curve.ScalarBaseMult(q.Bytes())
	return elliptic.Point{P1, P2}
}
