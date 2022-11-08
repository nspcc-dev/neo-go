package keys

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue1223(t *testing.T) {
	var d, x, y big.Int
	d.SetString("75066030006596498716801752450216843918658392116070031536027203512060270094427", 10)
	x.SetString("56810139335762307690884151098712528235297095596167964448512639328424930082240", 10)
	y.SetString("108055740278314806025442297642651169427004858252141003070998851291610422839293", 10)

	privateKey := PrivateKey{
		PrivateKey: ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: secp256k1.S256(),
				X:     &x,
				Y:     &y,
			},
			D: &d,
		},
	}
	pubKey := PublicKey(ecdsa.PublicKey{
		Curve: secp256k1.S256(),
		X:     privateKey.X,
		Y:     privateKey.Y,
	})

	hashedData := hash.Sha256([]byte("sample"))
	signature := privateKey.SignHash(hashedData)
	require.True(t, pubKey.Verify(signature, hashedData.BytesBE()))
}

func TestPubKeyVerify(t *testing.T) {
	var data = []byte("sample")
	hashedData := hash.Sha256(data)

	t.Run("Secp256r1", func(t *testing.T) {
		privKey, err := NewPrivateKey()
		assert.Nil(t, err)
		signedData := privKey.Sign(data)
		pubKey := privKey.PublicKey()
		result := pubKey.Verify(signedData, hashedData.BytesBE())
		expected := true
		assert.Equal(t, expected, result)

		// Small signature, no panic.
		assert.False(t, pubKey.Verify([]byte{1, 2, 3}, hashedData.BytesBE()))

		pubKey = &PublicKey{}
		assert.False(t, pubKey.Verify(signedData, hashedData.BytesBE()))
	})

	t.Run("Secp256k1", func(t *testing.T) {
		privateKey, err := NewSecp256k1PrivateKey()
		assert.Nil(t, err)
		signedData := privateKey.SignHash(hashedData)
		pubKey := privateKey.PublicKey()
		require.True(t, pubKey.Verify(signedData, hashedData.BytesBE()))

		pubKey = &PublicKey{}
		assert.False(t, pubKey.Verify(signedData, hashedData.BytesBE()))
	})
}

func TestWrongPubKey(t *testing.T) {
	sample := []byte("sample")
	hashedData := hash.Sha256(sample)

	t.Run("Secp256r1", func(t *testing.T) {
		privKey, _ := NewPrivateKey()
		signedData := privKey.Sign(sample)

		secondPrivKey, _ := NewPrivateKey()
		wrongPubKey := secondPrivKey.PublicKey()

		actual := wrongPubKey.Verify(signedData, hashedData.BytesBE())
		expcted := false
		assert.Equal(t, expcted, actual)
	})

	t.Run("Secp256k1", func(t *testing.T) {
		privateKey, err := NewSecp256k1PrivateKey()
		assert.Nil(t, err)
		signedData := privateKey.SignHash(hashedData)

		secondPrivKey, err := NewSecp256k1PrivateKey()
		assert.Nil(t, err)
		wrongPubKey := secondPrivKey.PublicKey()

		assert.False(t, wrongPubKey.Verify(signedData, hashedData.BytesBE()))
	})
}
