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
		witnessInfo, err := ntr.verifyIncompleteWitnesses(tx, nKeys)
		require.Error(t, err)
		require.Nil(t, witnessInfo)
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
		tx           *transaction.Transaction
		nKeys        uint8
		expectedInfo []witnessInfo
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
			nKeys: 1,
			expectedInfo: []witnessInfo{
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: Contract},
			},
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
						VerificationScript: sigScript2,
					},
					{
						InvocationScript:   sig,
						VerificationScript: sigScript3,
					},
					{},
				},
			},
			nKeys: 3,
			expectedInfo: []witnessInfo{
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc2.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc3.PublicKey()}},
				{typ: Contract},
			},
		},
		"single multisig 1 out of 3": {
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
			nKeys: 3,
			expectedInfo: []witnessInfo{
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Contract},
			},
		},
		"single multisig 2 out of 3": {
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
			nKeys: 3,
			expectedInfo: []witnessInfo{
				{typ: MultiSignature, nSigsLeft: 2, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Contract},
			},
		},
		"empty sig + single multisig 1 out of 3": {
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
			nKeys: 1 + 3,
			expectedInfo: []witnessInfo{
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Contract},
			},
		},
		"single multisig 1 out of 3 + empty single sig": {
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
			nKeys: 3 + 1,
			expectedInfo: []witnessInfo{
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: Contract},
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
			nKeys: 3 + 3,
			expectedInfo: []witnessInfo{
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: MultiSignature, nSigsLeft: 2, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Contract},
			},
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
			nKeys: 3 + 1,
			expectedInfo: []witnessInfo{
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: Contract},
			},
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
			nKeys: 1 + 3,
			expectedInfo: []witnessInfo{
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Contract},
			},
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
			nKeys: 3 + 1,
			expectedInfo: []witnessInfo{
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: Contract},
			},
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
			nKeys: 1 + 3,
			expectedInfo: []witnessInfo{
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Contract},
			},
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
			nKeys: 3 + 1,
			expectedInfo: []witnessInfo{
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: Contract},
			},
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
			nKeys: 1 + 3,
			expectedInfo: []witnessInfo{
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Contract},
			},
		},
		"multiple sigs + multiple multisigs": {
			tx: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: multisigScriptHash1},
					{Account: acc1.PublicKey().GetScriptHash()},
					{Account: acc2.PublicKey().GetScriptHash()},
					{Account: acc3.PublicKey().GetScriptHash()},
					{Account: multisigScriptHash2},
					{Account: notaryContractHash}},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   sig,
						VerificationScript: multisigScript1,
					},
					{
						InvocationScript:   sig,
						VerificationScript: sigScript1,
					},
					{
						InvocationScript:   []byte{},
						VerificationScript: sigScript2,
					},
					{
						InvocationScript:   sig,
						VerificationScript: sigScript3,
					},
					{
						InvocationScript:   []byte{},
						VerificationScript: multisigScript2,
					},
					{},
				},
			},
			nKeys: 3 + 1 + 1 + 1 + 3,
			expectedInfo: []witnessInfo{
				{typ: MultiSignature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc1.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc2.PublicKey()}},
				{typ: Signature, nSigsLeft: 1, pubs: keys.PublicKeys{acc3.PublicKey()}},
				{typ: MultiSignature, nSigsLeft: 2, pubs: keys.PublicKeys{acc1.PublicKey(), acc2.PublicKey(), acc3.PublicKey()}},
				{typ: Contract},
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			actualInfo, err := ntr.verifyIncompleteWitnesses(testCase.tx, testCase.nKeys)
			require.NoError(t, err)
			require.Equal(t, len(testCase.expectedInfo), len(actualInfo))
			for i, expected := range testCase.expectedInfo {
				actual := actualInfo[i]
				require.Equal(t, expected.typ, actual.typ)
				require.Equal(t, expected.nSigsLeft, actual.nSigsLeft)
				require.ElementsMatch(t, expected.pubs, actual.pubs)
				require.Nil(t, actual.sigs)
			}
		})
	}
}
