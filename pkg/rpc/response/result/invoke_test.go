package result

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestInvoke_MarshalJSON(t *testing.T) {
	result := &Invoke{
		State:          "HALT",
		GasConsumed:    237626000,
		Script:         []byte{10},
		Stack:          []stackitem.Item{stackitem.NewBigInteger(big.NewInt(1))},
		FaultException: "",
		// Transaction represents transaction bytes. Use GetTransaction method to decode it.
		Transaction: []byte{1, 2, 3, 4},
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
		"tx":"` + base64.StdEncoding.EncodeToString(result.Transaction) + `"
}`
	require.JSONEq(t, expected, string(data))

	actual := new(Invoke)
	require.NoError(t, json.Unmarshal(data, actual))
	require.Equal(t, result, actual)
}
