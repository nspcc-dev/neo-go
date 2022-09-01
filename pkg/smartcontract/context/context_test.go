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

type verifStub struct{}

func (v verifStub) Hash() util.Uint256                    { return util.Uint256{1, 2, 3} }
func (v verifStub) EncodeHashableFields() ([]byte, error) { return []byte{1}, nil }
func (v verifStub) DecodeHashableFields([]byte) error     { return nil }

func TestParameterContext_AddSignatureSimpleContract(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()
	tx := getContractTx(pub.GetScriptHash())
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

func TestGetCompleteTransactionForNonTx(t *testing.T) {
	c := NewParameterContext("Neo.Network.P2P.Payloads.Block", netmode.UnitTestNet, verifStub{})
	_, err := c.GetCompleteTransaction()
	require.Error(t, err)
}

func TestParameterContext_AddSignatureMultisig(t *testing.T) {
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
	tx := getContractTx(ctr.ScriptHash())
	c := NewParameterContext("Neo.Network.P2P.Payloads.Transaction", netmode.UnitTestNet, tx)
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	sig := priv.SignHashable(uint32(c.Network), tx)
	require.Error(t, c.AddSignature(ctr.ScriptHash(), ctr, priv.PublicKey(), sig))

	indices := []int{2, 3, 0, 1} // random order
	testSigWit := func(t *testing.T, num int) {
		t.Run("GetCompleteTransaction, bad", func(t *testing.T) {
			_, err := c.GetCompleteTransaction()
			require.Error(t, err)
		})
		for _, i := range indices[:num] {
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
		t.Run("GetCompleteTransaction, good", func(t *testing.T) {
			tx, err := c.GetCompleteTransaction()
			require.NoError(t, err)
			require.Equal(t, 1, len(tx.Scripts))
			scripts1 := make([]transaction.Witness, len(tx.Scripts))
			copy(scripts1, tx.Scripts)
			// Doing it twice shouldn't be a problem.
			tx, err = c.GetCompleteTransaction()
			require.NoError(t, err)
			require.Equal(t, scripts1, tx.Scripts)
		})
	}
	t.Run("exact number of sigs", func(t *testing.T) {
		testSigWit(t, 3)
	})
	t.Run("larger number of sigs", func(t *testing.T) {
		// Clean up.
		var itm = c.Items[ctr.ScriptHash()]
		for i := range itm.Parameters {
			itm.Parameters[i].Value = nil
		}
		itm.Signatures = make(map[string][]byte)
		testSigWit(t, 4)
	})
}

func newTestVM(w *transaction.Witness, tx *transaction.Transaction) *vm.VM {
	ic := &interop.Context{Network: uint32(netmode.UnitTestNet), Container: tx, Functions: crypto.Interops}
	v := ic.SpawnVM()
	v.LoadScript(w.VerificationScript)
	v.LoadScript(w.InvocationScript)
	return v
}

func TestParameterContext_MarshalJSON(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	tx := getContractTx(priv.GetScriptHash())
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
	t.Run("invalid hash", func(t *testing.T) {
		js := `{
		   "hash" : "0x0142f965b441b9af40a34b5cb24545b807c3ca24149201151fd93b204ea60e87",
		   "type" : "Neo.Core.ContractTransaction",
		   "items" : {
			  "0x60bd43f6e14dc19789296143b615e75cb73e19cc" : {
				 "parameters" : [
					{
					   "value" : "I4H7NpMj3xWczNNa31uZZDL7VvYNXrLHK6n2ARFCVVz/zW6ojrTtxgYpeFTMXfNwp+LULWjvJLQCxA6sky0yzQ==",
					   "type" : "Signature"
					}
				 ],
				 "signatures" : {
					"0268f0425415a67623e1e48ab3c3bd6275319c75e44358e4ec15abc6e50213b033" : "I4H7NpMj3xWczNNa31uZZDL7VvYNXrLHK6n2ARFCVVz/zW6ojrTtxgYpeFTMXfNwp+LULWjvJLQCxA6sky0yzQ=="
				 },
				 "script" : "DCECaPBCVBWmdiPh5Iqzw71idTGcdeRDWOTsFavG5QITsDNBVuezJw=="
			  }
		   },
		   "network" : 42,
		   "data" : "AMYrW54AAAAAAAAAAAAAAAAAAAAAAAAAAAEBAgMAAAAAAAAAAAAAAAAAAAAAAAAAARE="
}
`
		require.Error(t, json.Unmarshal([]byte(js), new(ParameterContext)))
	})
}

func TestSharpJSON(t *testing.T) {
	input := []byte(`{"type":"Neo.Network.P2P.Payloads.Transaction","hash":"0x71b519998f41bbc1d37e383e01e2e6efe84d65abf3c7279820cc7c63daa29448","data":"AKTv6hJY8h4AAAAAAKwiUwEAAAAA0lEAAAFBO\u002BhSRSuucNKVX2lk7k5Wdr\u002BkOQEAMR8RwB8MEHNldEV4ZWNGZWVGYWN0b3IMFHvGgcCh9x1UNFe2i7qNX5/dTl7MQWJ9W1I=","items":{"0x39a4bf76564eee64695f95d270ae2b4552e83b41":{"script":"GwwhAwCbdUDhDyVi5f2PrJ6uwlFmpYsm5BI0j/WoaSe/rCKiDCEDAgXpzvrqWh38WAryDI1aokaLsBSPGl5GBfxiLIDmBLoMIQIUuvDO6jpm8X5\u002BHoOeol/YvtbNgua7bmglAYkGX0T/AQwhAzjSoai75eQ8YzNBYTMIaaXgqqUeYTSWGEp8xylL\u002BVafDCEDPY41\u002BM2aM4UigLbZMJPHKS7VzpDZDxSfotpQumFo384MIQI\u002BmzLqiblNBm5kmxJP1Q45bukTaejipq4bEcFw0CIlbQwhA0CNzUFjlvZHg6xYfqHhWTxX2f6ogMimoZIOkqJZR3gGDCEDScfvC0qvGB8KPhNQxSexNsxbQkmMuDq4iAwF7ZUWfhwMIQJWZM7wq8uneHrV\u002BxLzrzHFzcekeQaKoq2O54gEdov/6QwhA1tPm\u002BK4U\u002BButaCcFn4Di5a0gEI1lhUQQjJS8u49u6WDDCEDZQpoRGGmS/Rr7lYdmYGkxXrcbMvTqVErg3AUgLMCGKsMIQJqEKorTXY5xd6vpP8IFGfbELXQBDJ0mipe4dK/7SPhwAwhAn5FmyZLb34yWrSwuw\u002BmQQgftoUX/WE\u002BvXqUy3nTCB5PDCECiMrUQqh3lgx2tPaI9L4w92glbZo9okkrAYC5EkORi08MIQKkDFUnmPeWNglYF\u002ByIkk/Gy3CU5aPLBZqbO8keo78NPQwhAqeDS\u002BmzLimB0VfLW706y0LP0R6lw7ECJNekTpjFkQ8bDCECuixw9ZlvNXpDGYcFhZ\u002BuLP6hPhFyligAdys9WIqdSr0MIQLVeGqSFKij8XV9dZb9EPUkEgXiwNaDYvR2ZXm6xhiSSQwhA9jVjSJXymyxRSK3ZRPUeD99SBgBaViTeUwhhlFcbedvDCEC23nmnFGK6SVOMUtvX0tj6RTN1LJXTcL5I2wBwfwdiXMMIQLsFD8AuIUkyvNqASHC3gnu8FGd2\u002BHHEKAPDiZjIB7kwAAVQZ7Q3Do=","parameters":[{"type":"Signature"},{"type":"Signature"},{"type":"Signature"},{"type":"Signature"},{"type":"Signature"},{"type":"Signature"},{"type":"Signature"},{"type":"Signature"},{"type":"Signature"},{"type":"Signature"},{"type":"Signature"}],"signatures":{"03650a684461a64bf46bee561d9981a4c57adc6ccbd3a9512b83701480b30218ab":"QtjYFNpGOOnij\u002BLwNZLOO3fHNoVQas\u002B4\u002BAo6SdvEeP3C12ATXzgPjAZrd5mCDc3KYkce0wwveEuuoYA8mhraUA==","0288cad442a877960c76b4f688f4be30f768256d9a3da2492b0180b91243918b4f":"RmuTXfPokXWEL9RIM9DqUUsOH8iRMfrKTp6LdhdJ0KBW6rNSEuxxNOpSUMBEW1EE2CNh1c\u002BmElj2Ny3o89SzGQ==","035b4f9be2b853e06eb5a09c167e038b96b4804235961510423252f2ee3dbba583":"1VYiT\u002BPe/7syYDSOWaJ1jPyZ6JDPrdU9toDu0Cg9pRQAJW1KLSexiosLA73k7lQeVbq4YuNlWnY7U8CYIQ/ilA==","02a40c552798f79636095817ec88924fc6cb7094e5a3cb059a9b3bc91ea3bf0d3d":"/mXUPXp/tI6Y7LhudKzBE8K2soHcPgrr48YLrwgbTI4qypYpOzh\u002BNj03pkAvk8\u002B68kuefevNQb/pjmPRvs80DA=="}}},"network":877933390}`)
	pc := ParameterContext{}
	require.NoError(t, json.Unmarshal(input, &pc))
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

func getContractTx(signer util.Uint160) *transaction.Transaction {
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx.Attributes = make([]transaction.Attribute, 0)
	tx.Scripts = make([]transaction.Witness, 0)
	tx.Signers = []transaction.Signer{{Account: signer}}
	tx.Hash()
	return tx
}
