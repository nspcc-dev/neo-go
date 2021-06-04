package keys

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/keytestcases"
	"github.com/stretchr/testify/assert"
)

func TestNEP2Encrypt(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		privKey, err := NewPrivateKeyFromHex(testCase.PrivateKey)
		if testCase.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.Nil(t, err)

		encryptedWif, err := NEP2Encrypt(privKey, testCase.Passphrase, NEP2ScryptParams())
		assert.Nil(t, err)

		assert.Equal(t, testCase.EncryptedWif, encryptedWif)
	}
}

func TestNEP2Decrypt(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		privKey, err := NEP2Decrypt(testCase.EncryptedWif, testCase.Passphrase, NEP2ScryptParams())
		if testCase.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.Nil(t, err)
		assert.Equal(t, testCase.PrivateKey, privKey.String())

		wif := privKey.WIF()
		assert.Equal(t, testCase.Wif, wif)

		address := privKey.Address()
		assert.Equal(t, testCase.Address, address)
	}
}

func TestNEP2DecryptErrors(t *testing.T) {
	p := "qwerty"

	// Not a base58-encoded value
	s := "qazwsx"
	_, err := NEP2Decrypt(s, p, NEP2ScryptParams())
	assert.Error(t, err)

	// Valid base58, but not a NEP-2 format.
	s = "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9o"
	_, err = NEP2Decrypt(s, p, NEP2ScryptParams())
	assert.Error(t, err)
}

func TestValidateNEP2Format(t *testing.T) {
	// Wrong length.
	s := []byte("gobbledygook")
	assert.Error(t, validateNEP2Format(s))

	// Wrong header 1.
	s = []byte("gobbledygookgobbledygookgobbledygookgob")
	assert.Error(t, validateNEP2Format(s))

	// Wrong header 2.
	s[0] = 0x01
	assert.Error(t, validateNEP2Format(s))

	// Wrong header 3.
	s[1] = 0x42
	assert.Error(t, validateNEP2Format(s))

	// OK
	s[2] = 0xe0
	assert.NoError(t, validateNEP2Format(s))
}
