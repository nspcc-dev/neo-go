package context

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestParameterContext_AddSignatureSimpleContract(t *testing.T) {
	tx := getContractTx()
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()
	sig := priv.SignHashable(uint32(netmode.UnitTestNet), tx)

	t.Run("invalid contract", func(t *testing.T) {
		c := NewParameterContext("Neo.Core.ContractTransaction", netmode.UnitTestNet, tx)
		ctr := &wallet.Contract{
			Script: pub.GetVerificationScript(),
			Parameters: []wallet.ContractParam{
				newParam(smartcontract.SignatureType, "parameter0"),
				newParam(smartcontract.SignatureType, "parameter1"),
			},
		}
		require.Error(t, c.AddSignature(ctr.ScriptHash(), ctr, pub, sig))
		if item := c.Items[ctr.ScriptHash()]; item != nil {
			require.Nil(t, item.Parameters[0].Value)
		}

		ctr.Parameters = ctr.Parameters[:0]
		require.Error(t, c.AddSignature(ctr.ScriptHash(), ctr, pub, sig))
		if item := c.Items[ctr.ScriptHash()]; item != nil {
			require.Nil(t, item.Parameters[0].Value)
		}
	})

	c := NewParameterContext("Neo.Core.ContractTransaction", netmode.UnitTestNet, tx)
	ctr := &wallet.Contract{
		Script:     pub.GetVerificationScript(),
		Parameters: []wallet.ContractParam{newParam(smartcontract.SignatureType, "parameter0")},
	}
	require.NoError(t, c.AddSignature(ctr.ScriptHash(), ctr, pub, sig))
	item := c.Items[ctr.ScriptHash()]
	require.NotNil(t, item)
	require.Equal(t, sig, item.Parameters[0].Value)

	t.Run("GetWitness", func(t *testing.T) {
		w, err := c.GetWitness(ctr.ScriptHash())
		require.NoError(t, err)
		v := newTestVM(w, tx)
		require.NoError(t, v.Run())
		require.Equal(t, 1, v.Estack().Len())
		require.Equal(t, true, v.Estack().Pop().Value())
	})
	t.Run("not found", func(t *testing.T) {
		ctr := &wallet.Contract{
			Script:     []byte{byte(opcode.DROP), byte(opcode.PUSHT)},
			Parameters: []wallet.ContractParam{newParam(smartcontract.SignatureType, "parameter0")},
		}
		_, err := c.GetWitness(ctr.ScriptHash())
		require.Error(t, err)
	})
}

func TestParameterContext_AddSignatureMultisig(t *testing.T) {
	tx := getContractTx()
	c := NewParameterContext("Neo.Core.ContractTransaction", netmode.UnitTestNet, tx)
	privs, pubs := getPrivateKeys(t, 4)
	pubsCopy := keys.PublicKeys(pubs).Copy()
	script, err := smartcontract.CreateMultiSigRedeemScript(3, pubsCopy)
	require.NoError(t, err)

	ctr := &wallet.Contract{
		Script: script,
		Parameters: []wallet.ContractParam{
			newParam(smartcontract.SignatureType, "parameter0"),
			newParam(smartcontract.SignatureType, "parameter1"),
			newParam(smartcontract.SignatureType, "parameter2"),
		},
	}
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	sig := priv.SignHashable(uint32(c.Network), tx)
	require.Error(t, c.AddSignature(ctr.ScriptHash(), ctr, priv.PublicKey(), sig))

	indices := []int{2, 3, 0} // random order
	for _, i := range indices {
		sig := privs[i].SignHashable(uint32(c.Network), tx)
		require.NoError(t, c.AddSignature(ctr.ScriptHash(), ctr, pubs[i], sig))
		require.Error(t, c.AddSignature(ctr.ScriptHash(), ctr, pubs[i], sig))

		item := c.Items[ctr.ScriptHash()]
		require.NotNil(t, item)
		require.Equal(t, sig, item.GetSignature(pubs[i]))
	}

	t.Run("GetWitness", func(t *testing.T) {
		w, err := c.GetWitness(ctr.ScriptHash())
		require.NoError(t, err)
		v := newTestVM(w, tx)
		require.NoError(t, v.Run())
		require.Equal(t, 1, v.Estack().Len())
		require.Equal(t, true, v.Estack().Pop().Value())
	})
}

func newTestVM(w *transaction.Witness, tx *transaction.Transaction) *vm.VM {
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), Container: tx}
	crypto.Register(ic)
	v := ic.SpawnVM()
	v.LoadScript(w.VerificationScript)
	v.LoadScript(w.InvocationScript)
	return v
}

func TestParameterContext_MarshalJSON(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	tx := getContractTx()
	sign := priv.SignHashable(uint32(netmode.UnitTestNet), tx)

	expected := &ParameterContext{
		Type:       "Neo.Core.ContractTransaction",
		Network:    netmode.UnitTestNet,
		Verifiable: tx,
		Items: map[util.Uint160]*Item{
			priv.GetScriptHash(): {
				Script: priv.PublicKey().GetVerificationScript(),
				Parameters: []smartcontract.Parameter{{
					Type:  smartcontract.SignatureType,
					Value: sign,
				}},
				Signatures: map[string][]byte{
					hex.EncodeToString(priv.PublicKey().Bytes()): sign,
				},
			},
		},
	}

	testserdes.MarshalUnmarshalJSON(t, expected, new(ParameterContext))

	t.Run("invalid script", func(t *testing.T) {
		js := `{
 			"script": "AQID",
 			"parameters": [
  				{
   					"type": "Signature",
   					"value": "QfOZLLqjMyPWMzRxMAKw7fcd8leLcpwiiTV2pUyC0pth/y7Iw7o7WzNpxeAJm5bmExmlF7g5pMhXz1xVT6KK3g=="
  				}
			],
 			"signatures": {
				"025c210bde738e0e646929ee04ec2ccb42a700356083f55386b5347b9b725c10b9": "a6c6d8a2334791888df559419f07209ee39e2f20688af8cc38010854b98abf77194e37f173bbc86b77dce4afa8ce3ae5170dd346b5265bcb9b723d83299a6f0f",
  				"035d4da640b3a39f19ed88855aeddd97725422b4230ccae56bd5544419d0056ea9": "058e577f23395f382194eebb83f66bb8903c8f3c5b6afd759c20f2518466124dcd9cbccfc029a42e9a7d5a3a060b091edc73dcac949fd894d7a9d10678296ac6"
		}`
		require.Error(t, json.Unmarshal([]byte(js), new(ParameterContext)))
	})
}

func getPrivateKeys(t *testing.T, n int) ([]*keys.PrivateKey, []*keys.PublicKey) {
	privs := make([]*keys.PrivateKey, n)
	pubs := make([]*keys.PublicKey, n)
	for i := range privs {
		var err error
		privs[i], err = keys.NewPrivateKey()
		require.NoError(t, err)
		pubs[i] = privs[i].PublicKey()
	}
	return privs, pubs
}

func newParam(typ smartcontract.ParamType, name string) wallet.ContractParam {
	return wallet.ContractParam{
		Name: name,
		Type: typ,
	}
}

func getContractTx() *transaction.Transaction {
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx.Attributes = make([]transaction.Attribute, 0)
	tx.Scripts = make([]transaction.Witness, 0)
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Hash()
	return tx
}
