package wallet

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/internal/keytestcases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccount(t *testing.T) {
	acc, err := NewAccount()
	require.NoError(t, err)
	require.NotNil(t, acc)
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

func TestNewAccountFromEncryptedWIF(t *testing.T) {
	for _, tc := range keytestcases.Arr {
		acc, err := NewAccountFromEncryptedWIF(tc.EncryptedWif, tc.Passphrase)
		if tc.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		compareFields(t, tc, acc)
	}
}

func TestContract_MarshalJSON(t *testing.T) {
	var c Contract

	data := []byte(`{"script":"0102","parameters":[{"name":"name0", "type":"Signature"}],"deployed":false}`)
	require.NoError(t, json.Unmarshal(data, &c))
	require.Equal(t, []byte{1, 2}, c.Script)

	result, err := json.Marshal(c)
	require.NoError(t, err)
	require.JSONEq(t, string(data), string(result))

	data = []byte(`1`)
	require.Error(t, json.Unmarshal(data, &c))

	data = []byte(`{"script":"ERROR","parameters":[1],"deployed":false}`)
	require.Error(t, json.Unmarshal(data, &c))
}

func TestContract_ScriptHash(t *testing.T) {
	script := []byte{0, 1, 2, 3}
	c := &Contract{Script: script}

	require.Equal(t, hash.Hash160(script), c.ScriptHash())
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
