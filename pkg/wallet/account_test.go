package wallet

import (
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/internal/keytestcases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccount(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		acc, err := NewAccountFromWIF(testCase.Wif)
		if testCase.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		compareFields(t, testCase, acc)
	}
}

func TestDecryptAccount(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		acc := &Account{EncryptedWIF: testCase.EncryptedWif}
		assert.Nil(t, acc.PrivateKey())
		err := acc.Decrypt(testCase.Passphrase)
		if testCase.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		assert.NotNil(t, acc.PrivateKey())
		assert.Equal(t, testCase.PrivateKey, acc.privateKey.String())
	}
	// No encrypted key.
	acc := &Account{}
	require.Error(t, acc.Decrypt("qwerty"))
}

func TestNewFromWif(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		acc, err := NewAccountFromWIF(testCase.Wif)
		if testCase.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		compareFields(t, testCase, acc)
	}
}

func compareFields(t *testing.T, tk keytestcases.Ktype, acc *Account) {
	if want, have := tk.Address, acc.Address; want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.Wif, acc.wif; want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.PublicKey, hex.EncodeToString(acc.publicKey); want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.PrivateKey, acc.privateKey.String(); want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
}
