package context

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestParameterContext_MarshalJSON(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	tx := getContractTx()
	data := tx.GetSignedPart()
	sign := priv.Sign(data)

	expected := &ParameterContext{
		Type:       "Neo.Core.ContractTransaction",
		Verifiable: tx,
		Items: map[util.Uint160]*Item{
			priv.GetScriptHash(): {
				Script: priv.GetScriptHash(),
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

	data, err = json.Marshal(expected)
	require.NoError(t, err)

	actual := new(ParameterContext)
	require.NoError(t, json.Unmarshal(data, actual))
	require.Equal(t, expected, actual)
}

func getContractTx() *transaction.Transaction {
	tx := transaction.NewContractTX()
	tx.AddInput(&transaction.Input{
		PrevHash:  util.Uint256{1, 2, 3, 4},
		PrevIndex: 5,
	})
	tx.AddOutput(&transaction.Output{
		AssetID:    util.Uint256{7, 8, 9},
		Amount:     10,
		ScriptHash: util.Uint160{11, 12},
	})
	tx.Data = new(transaction.ContractTX)
	tx.Attributes = make([]transaction.Attribute, 0)
	tx.Scripts = make([]transaction.Witness, 0)
	tx.Hash()
	return tx
}
