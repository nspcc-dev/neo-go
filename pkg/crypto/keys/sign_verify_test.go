package keys

import (
	"crypto/ecdsa"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

		pubKey = &PublicKey{}
		assert.False(t, pubKey.Verify(signedData, hashedData.BytesBE()))
	})

	t.Run("Secp256k1", func(t *testing.T) {
		privateKey, err := btcec.NewPrivateKey(btcec.S256())
		assert.Nil(t, err)
		signature, err := privateKey.Sign(hashedData.BytesBE())
		require.NoError(t, err)
		signedData := append(signature.R.Bytes(), signature.S.Bytes()...)
		pubKey := PublicKey(ecdsa.PublicKey{
			Curve: btcec.S256(),
			X:     privateKey.X,
			Y:     privateKey.Y,
		})
		require.True(t, pubKey.Verify(signedData, hashedData.BytesBE()))

		pubKey = PublicKey{}
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
		privateKey, err := btcec.NewPrivateKey(btcec.S256())
		assert.Nil(t, err)
		signature, err := privateKey.Sign(hashedData.BytesBE())
		assert.Nil(t, err)
		signedData := append(signature.R.Bytes(), signature.S.Bytes()...)

		secondPrivKey, err := btcec.NewPrivateKey(btcec.S256())
		assert.Nil(t, err)
		wrongPubKey := PublicKey(ecdsa.PublicKey{
			Curve: btcec.S256(),
			X:     secondPrivKey.X,
			Y:     secondPrivKey.Y,
		})

		assert.False(t, wrongPubKey.Verify(signedData, hashedData.BytesBE()))
	})
}
