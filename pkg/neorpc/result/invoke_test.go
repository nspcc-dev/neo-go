package result

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/holiman/uint256"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/require"
)

func TestInvoke_MarshalJSON(t *testing.T) {
	tx := transaction.New([]byte{1, 2, 3, 4}, 0)
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	tx.Scripts = []transaction.Witness{{InvocationScript: []byte{}, VerificationScript: []byte{}}}
	_ = tx.Size()
	tx.Hash()

	result := &Invoke{
		State:          "HALT",
		GasConsumed:    237626000,
		Script:         []byte{10},
		Stack:          []stackitem.Item{stackitem.NewBigInteger(uint256.NewInt(1))},
		FaultException: "",
		Notifications:  []state.NotificationEvent{},
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
		"notifications":[],
		"exception": null,
		"tx":"` + base64.StdEncoding.EncodeToString(tx.Bytes()) + `"
}`
	require.JSONEq(t, expected, string(data))

	actual := new(Invoke)
	require.NoError(t, json.Unmarshal(data, actual))
	require.Equal(t, result, actual)
}

func TestAppExecToInvocation(t *testing.T) {
	// With error.
	someErr := errors.New("some err")
	_, err := AppExecToInvocation(nil, someErr)
	require.ErrorIs(t, err, someErr)

	// Good.
	h := util.Uint256{1, 2, 3}
	ex := state.Execution{
		Trigger:     trigger.Application,
		VMState:     vmstate.Fault,
		GasConsumed: 123,
		Stack:       []stackitem.Item{stackitem.NewBigInteger(uint256.NewInt(123))},
		Events: []state.NotificationEvent{{
			ScriptHash: util.Uint160{3, 2, 1},
			Name:       "Notification",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.Null{}}),
		}},
		FaultException: "some fault exception",
	}
	inv, err := AppExecToInvocation(&state.AppExecResult{
		Container: h,
		Execution: ex,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, ex.VMState.String(), inv.State)
	require.Equal(t, ex.GasConsumed, inv.GasConsumed)
	require.Nil(t, inv.Script)
	require.Equal(t, ex.Stack, inv.Stack)
	require.Equal(t, ex.FaultException, inv.FaultException)
	require.Equal(t, ex.Events, inv.Notifications)
	require.Nil(t, inv.Transaction)
	require.Nil(t, inv.Diagnostics)
	require.Equal(t, uuid.UUID{}, inv.Session)
}
