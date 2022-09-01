package wallet

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/keytestcases"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccount(t *testing.T) {
	acc, err := NewAccount()
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Equal(t, acc.Address, address.Uint160ToString(acc.ScriptHash()))
}

func TestDecryptAccount(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		acc := &Account{EncryptedWIF: testCase.EncryptedWif}
		assert.Nil(t, acc.PrivateKey())
		err := acc.Decrypt(testCase.Passphrase, keys.NEP2ScryptParams())
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
	require.Error(t, acc.Decrypt("qwerty", keys.NEP2ScryptParams()))
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
		acc, err := NewAccountFromEncryptedWIF(tc.EncryptedWif, tc.Passphrase, keys.NEP2ScryptParams())
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

	data := []byte(`{"script":"AQI=","parameters":[{"name":"name0", "type":"Signature"}],"deployed":false}`)
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

func TestContractSignTx(t *testing.T) {
	acc, err := NewAccount()
	require.NoError(t, err)
	require.True(t, acc.CanSign())

	accNoContr := *acc
	accNoContr.Contract = nil
	tx := &transaction.Transaction{
		Script: []byte{1, 2, 3},
		Signers: []transaction.Signer{{
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		}},
	}
	require.Error(t, accNoContr.SignTx(0, tx))

	acc2, err := NewAccount()
	require.NoError(t, err)
	require.True(t, acc2.CanSign())

	require.Error(t, acc2.SignTx(0, tx))

	pubs := keys.PublicKeys{acc.PublicKey(), acc2.PublicKey()}
	multiS, err := smartcontract.CreateDefaultMultiSigRedeemScript(pubs)
	require.NoError(t, err)
	multiAcc := NewAccountFromPrivateKey(acc.privateKey)
	require.NoError(t, multiAcc.ConvertMultisig(2, pubs))
	multiAcc2 := NewAccountFromPrivateKey(acc2.privateKey)
	require.NoError(t, multiAcc2.ConvertMultisig(2, pubs))

	tx = &transaction.Transaction{
		Script: []byte{1, 2, 3},
		Signers: []transaction.Signer{{
			Account: acc2.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		}, {
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.None,
		}, {
			Account: hash.Hash160(multiS),
			Scopes:  transaction.None,
		}},
	}
	require.Error(t, acc.SignTx(0, tx)) // Can't append, no witness for acc2.

	require.NoError(t, acc2.SignTx(0, tx)) // Append script for acc2.
	require.Equal(t, 1, len(tx.Scripts))
	require.Equal(t, 66, len(tx.Scripts[0].InvocationScript))

	require.NoError(t, acc2.SignTx(0, tx)) // Sign again, effectively a no-op.
	require.Equal(t, 1, len(tx.Scripts))
	require.Equal(t, 66, len(tx.Scripts[0].InvocationScript))

	acc2.Locked = true
	require.False(t, acc2.CanSign())
	require.Error(t, acc2.SignTx(0, tx))     // Locked account.
	require.Nil(t, acc2.PublicKey())         // Locked account.
	require.Nil(t, acc2.SignHashable(0, tx)) // Locked account.

	acc2.Locked = false
	acc2.Close()
	require.False(t, acc2.CanSign())
	require.Error(t, acc2.SignTx(0, tx)) // No private key.
	acc2.Close()                         // No-op.
	require.False(t, acc2.CanSign())

	tx.Scripts = append(tx.Scripts, transaction.Witness{
		VerificationScript: acc.Contract.Script,
	})
	require.NoError(t, acc.SignTx(0, tx)) // Add invocation script for existing witness.
	require.Equal(t, 66, len(tx.Scripts[1].InvocationScript))
	require.NotNil(t, acc.SignHashable(0, tx)) // Works via Hashable too.

	require.NoError(t, multiAcc.SignTx(0, tx))
	require.Equal(t, 3, len(tx.Scripts))
	require.Equal(t, 66, len(tx.Scripts[2].InvocationScript))

	require.NoError(t, multiAcc2.SignTx(0, tx)) // Append to existing script.
	require.Equal(t, 3, len(tx.Scripts))
	require.Equal(t, 132, len(tx.Scripts[2].InvocationScript))
}

func TestContract_ScriptHash(t *testing.T) {
	script := []byte{0, 1, 2, 3}
	c := &Contract{Script: script}

	require.Equal(t, hash.Hash160(script), c.ScriptHash())
}

func TestAccount_ConvertMultisig(t *testing.T) {
	// test is based on a wallet1_solo.json accounts from neo-local
	a, err := NewAccountFromWIF("KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY")
	require.NoError(t, err)

	hexs := []string{
		"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2", // <- this is our key
		"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
		"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
	}

	t.Run("locked", func(t *testing.T) {
		a.Locked = true
		pubs := convertPubs(t, hexs)
		require.Error(t, a.ConvertMultisig(1, pubs))
		a.Locked = false
	})
	t.Run("no private key", func(t *testing.T) {
		pk := a.privateKey
		a.privateKey = nil
		pubs := convertPubs(t, hexs)
		require.Error(t, a.ConvertMultisig(0, pubs))
		a.privateKey = pk
	})
	t.Run("invalid number of signatures", func(t *testing.T) {
		pubs := convertPubs(t, hexs)
		require.Error(t, a.ConvertMultisig(0, pubs))
	})

	t.Run("account key is missing from multisig", func(t *testing.T) {
		pubs := convertPubs(t, hexs[1:])
		require.Error(t, a.ConvertMultisig(1, pubs))
	})

	t.Run("1/1 multisig", func(t *testing.T) {
		pubs := convertPubs(t, hexs[:1])
		require.NoError(t, a.ConvertMultisig(1, pubs))
		require.Equal(t, "NfgHwwTi3wHAS8aFAN243C5vGbkYDpqLHP", a.Address)
	})

	t.Run("3/4 multisig", func(t *testing.T) {
		pubs := convertPubs(t, hexs)
		require.NoError(t, a.ConvertMultisig(3, pubs))
		require.Equal(t, "NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq", a.Address)
	})
}

func convertPubs(t *testing.T, hexKeys []string) []*keys.PublicKey {
	pubs := make([]*keys.PublicKey, len(hexKeys))
	for i := range pubs {
		var err error
		pubs[i], err = keys.NewPublicKeyFromString(hexKeys[i])
		require.NoError(t, err)
	}
	return pubs
}

func compareFields(t *testing.T, tk keytestcases.Ktype, acc *Account) {
	want, have := tk.Address, acc.Address
	require.Equalf(t, want, have, "expected address %s got %s", want, have)
	want, have = tk.Wif, acc.privateKey.WIF()
	require.Equalf(t, want, have, "expected wif %s got %s", want, have)
	want, have = tk.PublicKey, hex.EncodeToString(acc.PublicKey().Bytes())
	require.Equalf(t, want, have, "expected pub key %s got %s", want, have)
	want, have = tk.PrivateKey, acc.privateKey.String()
	require.Equalf(t, want, have, "expected priv key %s got %s", want, have)
}
