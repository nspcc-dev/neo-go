package result

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestInvoke_MarshalJSON(t *testing.T) {
	tx := transaction.New([]byte{1, 2, 3, 4}, 0)
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Scripts = []transaction.Witness{transaction.Witness{InvocationScript: []byte{}, VerificationScript: []byte{}}}
	_ = tx.Size()
	tx.Hash()

	result := &Invoke{
		State:          "HALT",
		GasConsumed:    237626000,
		Script:         []byte{10},
		Stack:          []stackitem.Item{stackitem.NewBigInteger(big.NewInt(1))},
		FaultException: "",
		Transaction:    tx,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)
	expected := `{
		"state":"HALT",
		"gasconsumed":"237626000",
		"script":"` + base64.StdEncoding.EncodeToString(result.Script) + `",
		"stack":[
			{"type":"Integer","value":"1"}
		],
		"tx":"` + base64.StdEncoding.EncodeToString(tx.Bytes()) + `"
}`
	require.JSONEq(t, expected, string(data))

	actual := new(Invoke)
	require.NoError(t, json.Unmarshal(data, actual))
	require.Equal(t, result, actual)
}
