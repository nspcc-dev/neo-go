package notary

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestWallet(t *testing.T) {
	bc := fakechain.NewFakeChain()
	mainCfg := config.P2PNotary{Enabled: true}
	cfg := Config{
		MainCfg: mainCfg,
		Chain:   bc,
		Log:     zaptest.NewLogger(t),
	}
	t.Run("unexisting wallet", func(t *testing.T) {
		cfg.MainCfg.UnlockWallet.Path = "./testdata/does_not_exists.json"
		_, err := NewNotary(cfg, netmode.UnitTestNet, mempool.New(1, 1, true), nil)
		require.Error(t, err)
	})

	t.Run("bad password", func(t *testing.T) {
		cfg.MainCfg.UnlockWallet.Path = "./testdata/notary1.json"
		cfg.MainCfg.UnlockWallet.Password = "invalid"
		_, err := NewNotary(cfg, netmode.UnitTestNet, mempool.New(1, 1, true), nil)
		require.Error(t, err)
	})

	t.Run("good", func(t *testing.T) {
		cfg.MainCfg.UnlockWallet.Path = "./testdata/notary1.json"
		cfg.MainCfg.UnlockWallet.Password = "one"
		_, err := NewNotary(cfg, netmode.UnitTestNet, mempool.New(1, 1, true), nil)
		require.NoError(t, err)
	})
}

func TestVerifyIncompleteRequest(t *testing.T) {
	bc := fakechain.NewFakeChain()
	notaryContractHash := util.Uint160{1, 2, 3}
	bc.NotaryContractScriptHash = notaryContractHash
	_, ntr, _ := getTestNotary(t, bc, "./testdata/notary1.json", "one")
	sig := append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64)...) // we're not interested in signature correctness
	acc1, _ := keys.NewPrivateKey()
	acc2, _ := keys.NewPrivateKey()
	acc3, _ := keys.NewPrivateKey()
	sigScript1 := acc1.PublicKey().GetVerificationScript()
	sigScript2 := acc2.PublicKey().GetVerificationScript()
	sigScript3 := acc3.PublicKey().GetVerificationScript()
	multisigScript1, err := smartcontract.CreateMultiSigRedeemScript(1, keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()})
	require.NoError(t, err)
	multisigScriptHash1 := hash.Hash160(multisigScript1)
	multisigScript2, err := smartcontract.CreateMultiSigRedeemScript(2, keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()})
	require.NoError(t, err)
	multisigScriptHash2 := hash.Hash160(multisigScript2)

	checkErr := func(t *testing.T, tx *transaction.Transaction, nKeys uint8) {
		typ, nSigs, pubs, err := ntr.verifyIncompleteWitnesses(tx, nKeys)
		require.Error(t, err)
		require.Equal(t, Unknown, typ)
		require.Equal(t, uint8(0), nSigs)
		require.Nil(t, pubs)
	}

	errCases := map[string]struct {
		tx    *transaction.Transaction
		nKeys uint8
	}{
		"not enough signers": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: notaryContractHash}},
				Scripts: []transaction.Witness{{}},
			},
		},
		"missing Notary witness": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.GetScriptHash()}, {Account: acc2.GetScriptHash()}},
				Scripts: []transaction.Witness{{}, {}},
			},
		},
		"unknown witness type": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.PublicKey().GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{},
					{},
				},
			},
		},
		"bad verification script": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.PublicKey().GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   []byte{},
						VerificationScript: []byte{1, 2, 3},
					},
					{},
				},
			},
		},
		"several multisig witnesses": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1}, {Account: multisigScriptHash2}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript2,
					},
					{},
				},
			},
			nKeys: 2,
		},
		"multisig + sig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1}, {Account: acc1.PublicKey().GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{
						InvocationScript:   sig,
						VerificationScript: sigScript1,
					},
					{},
				},
			},
			nKeys: 2,
		},
		"sig + multisig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.PublicKey().GetScriptHash()}, {Account: multisigScriptHash1}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: sigScript1,
					},
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{},
				},
			},
			nKeys: 2,
		},
		"empty multisig + sig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1}, {Account: acc1.PublicKey().GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   []byte{},
						VerificationScript: multisigScript1,
					},
					{
						InvocationScript:   sig,
						VerificationScript: sigScript1,
					},
					{},
				},
			},
			nKeys: 2,
		},
		"sig + empty multisig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.PublicKey().GetScriptHash()}, {Account: multisigScriptHash1}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: sigScript1,
					},
					{
						InvocationScript:   []byte{},
						VerificationScript: multisigScript1,
					},
					{},
				},
			},
			nKeys: 2,
		},
		"multisig + empty sig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1}, {Account: acc1.PublicKey().GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{
						InvocationScript:   []byte{},
						VerificationScript: sigScript1,
					},
					{},
				},
			},
			nKeys: 2,
		},
		"empty sig + multisig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.PublicKey().GetScriptHash()}, {Account: multisigScriptHash1}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   []byte{},
						VerificationScript: sigScript1,
					},
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{},
				},
			},
			nKeys: 2,
		},
		"sig: bad nKeys": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.PublicKey().GetScriptHash()}, {Account: acc2.PublicKey().GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: sigScript1,
					},
					{
						InvocationScript:   sig,
						VerificationScript: sigScript2,
					},
					{},
				},
			},
			nKeys: 3,
		},
		"multisig: bad witnesses count": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
				},
			},
			nKeys: 2,
		},
		"multisig: bad nKeys": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{},
				},
			},
			nKeys: 2,
		},
	}

	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			checkErr(t, errCase.tx, errCase.nKeys)
		})
	}

	testCases := map[string]struct {
		tx            *transaction.Transaction
		nKeys         uint8
		expectedType  RequestType
		expectedNSigs uint8
		expectedPubs  keys.PublicKeys
	}{
		"single sig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: sigScript1,
					},
					{},
				},
			},
			nKeys:         1,
			expectedType:  Signature,
			expectedNSigs: 1,
		},
		"multiple sig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.GetScriptHash()}, {Account: acc2.GetScriptHash()}, {Account: acc3.GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: sigScript1,
					},
					{
						InvocationScript:   []byte{},
						VerificationScript: []byte{},
					},
					{
						InvocationScript:   sig,
						VerificationScript: sigScript3,
					},
					{},
				},
			},
			nKeys:         3,
			expectedType:  Signature,
			expectedNSigs: 3,
		},
		"multisig 1 out of 3": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{},
				},
			},
			nKeys:         3,
			expectedType:  MultiSignature,
			expectedNSigs: 1,
			expectedPubs:  keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()},
		},
		"multisig 2 out of 3": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash2}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript2,
					},
					{},
				},
			},
			nKeys:         3,
			expectedType:  MultiSignature,
			expectedNSigs: 2,
			expectedPubs:  keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()},
		},
		"empty + multisig": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: acc1.PublicKey().GetScriptHash()}, {Account: multisigScriptHash1}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   []byte{},
						VerificationScript: []byte{},
					},
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{},
				},
			},
			nKeys:         3,
			expectedType:  MultiSignature,
			expectedNSigs: 1,
			expectedPubs:  keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()},
		},
		"multisig + empty": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1}, {Account: acc1.PublicKey().GetScriptHash()}, {Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{
						InvocationScript:   []byte{},
						VerificationScript: []byte{},
					},
					{},
				},
			},
			nKeys:         3,
			expectedType:  MultiSignature,
			expectedNSigs: 1,
			expectedPubs:  keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			typ, nSigs, pubs, err := ntr.verifyIncompleteWitnesses(testCase.tx, testCase.nKeys)
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedType, typ)
			assert.Equal(t, testCase.expectedNSigs, nSigs)
			assert.ElementsMatch(t, testCase.expectedPubs, pubs)
		})
	}
}
