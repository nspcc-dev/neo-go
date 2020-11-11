package state

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeNotificationEvent(t *testing.T) {
	event := &NotificationEvent{
		ScriptHash: random.Uint160(),
		Name:       "Event",
		Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewBool(true)}),
	}

	testserdes.EncodeDecodeBinary(t, event, new(NotificationEvent))
}

func TestEncodeDecodeAppExecResult(t *testing.T) {
	t.Run("halt", func(t *testing.T) {
		appExecResult := &AppExecResult{
			Container: random.Uint256(),
			Execution: Execution{
				Trigger:     1,
				VMState:     vm.HaltState,
				GasConsumed: 10,
				Stack:       []stackitem.Item{},
				Events:      []NotificationEvent{},
			},
		}

		testserdes.EncodeDecodeBinary(t, appExecResult, new(AppExecResult))
	})
	t.Run("fault", func(t *testing.T) {
		appExecResult := &AppExecResult{
			Container: random.Uint256(),
			Execution: Execution{
				Trigger:        1,
				VMState:        vm.FaultState,
				GasConsumed:    10,
				Stack:          []stackitem.Item{},
				Events:         []NotificationEvent{},
				FaultException: "unhandled error",
			},
		}

		testserdes.EncodeDecodeBinary(t, appExecResult, new(AppExecResult))
	})
}

func TestMarshalUnmarshalJSONNotificationEvent(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		ne := &NotificationEvent{
			ScriptHash: random.Uint160(),
			Name:       "my_ne",
			Item: stackitem.NewArray([]stackitem.Item{
				stackitem.NewBool(true),
			}),
		}
		testserdes.MarshalUnmarshalJSON(t, ne, new(NotificationEvent))
	})

	t.Run("MarshalJSON recursive reference", func(t *testing.T) {
		i := make([]stackitem.Item, 1)
		recursive := stackitem.NewArray(i)
		i[0] = recursive
		ne := &NotificationEvent{
			Item: recursive,
		}
		_, err := json.Marshal(ne)
		require.NoError(t, err)
	})

	t.Run("UnmarshalJSON error", func(t *testing.T) {
		errorCases := []string{
			`{"contract":"0xBadHash","eventname":"my_ne","state":{"type":"Array","value":[{"type":"Boolean","value":true}]}}`,
			`{"contract":"0xab2f820e2aa7cca1e081283c58a7d7943c33a2f1","eventname":"my_ne","state":{"type":"Array","value":[{"type":"BadType","value":true}]}}`,
			`{"contract":"0xab2f820e2aa7cca1e081283c58a7d7943c33a2f1","eventname":"my_ne","state":{"type":"Boolean", "value":true}}`,
		}
		for _, errCase := range errorCases {
			err := json.Unmarshal([]byte(errCase), new(NotificationEvent))
			require.Error(t, err)
		}

	})
}

func TestMarshalUnmarshalJSONAppExecResult(t *testing.T) {
	t.Run("positive, transaction", func(t *testing.T) {
		appExecResult := &AppExecResult{
			Container: random.Uint256(),
			Execution: Execution{
				Trigger:     trigger.Application,
				VMState:     vm.HaltState,
				GasConsumed: 10,
				Stack:       []stackitem.Item{},
				Events:      []NotificationEvent{},
			},
		}
		testserdes.MarshalUnmarshalJSON(t, appExecResult, new(AppExecResult))
	})

	t.Run("positive, fault state", func(t *testing.T) {
		appExecResult := &AppExecResult{
			Container: random.Uint256(),
			Execution: Execution{
				Trigger:        trigger.Application,
				VMState:        vm.FaultState,
				GasConsumed:    10,
				Stack:          []stackitem.Item{},
				Events:         []NotificationEvent{},
				FaultException: "unhandled exception",
			},
		}
		testserdes.MarshalUnmarshalJSON(t, appExecResult, new(AppExecResult))
	})
	t.Run("positive, block", func(t *testing.T) {
		appExecResult := &AppExecResult{
			Container: random.Uint256(),
			Execution: Execution{
				Trigger:     trigger.OnPersist,
				VMState:     vm.HaltState,
				GasConsumed: 10,
				Stack:       []stackitem.Item{},
				Events:      []NotificationEvent{},
			},
		}
		data, err := json.Marshal(appExecResult)
		require.NoError(t, err)
		actual := new(AppExecResult)
		require.NoError(t, json.Unmarshal(data, actual))
		expected := &AppExecResult{
			// we have no way to restore block hash as it was not marshalled
			Container: util.Uint256{},
			Execution: Execution{
				Trigger:     appExecResult.Trigger,
				VMState:     appExecResult.VMState,
				GasConsumed: appExecResult.GasConsumed,
				Stack:       appExecResult.Stack,
				Events:      appExecResult.Events,
			},
		}
		require.Equal(t, expected, actual)
	})

	t.Run("MarshalJSON recursive reference", func(t *testing.T) {
		i := make([]stackitem.Item, 1)
		recursive := stackitem.NewArray(i)
		i[0] = recursive
		errorCases := []*AppExecResult{
			{
				Execution: Execution{
					Stack: i,
				},
			},
		}
		for _, errCase := range errorCases {
			_, err := json.Marshal(errCase)
			require.NoError(t, err)
		}
	})

	t.Run("UnmarshalJSON error", func(t *testing.T) {
		nilStackCases := []string{
			`{"txid":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","trigger":"Application","vmstate":"HALT","gasconsumed":"1","stack":[{"type":"WrongType","value":"1"}],"notifications":[]}`,
		}
		for _, str := range nilStackCases {
			actual := new(AppExecResult)
			err := json.Unmarshal([]byte(str), actual)
			require.NoError(t, err)
			require.Nil(t, actual.Stack)
		}

		errorCases := []string{
			`{"txid":"0xBadHash","trigger":"Application","vmstate":"HALT","gasconsumed":"1","stack":[{"type":"Integer","value":"1"}],"notifications":[]}`,
			`{"txid":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","trigger":"Application","vmstate":"BadState","gasconsumed":"1","stack":[{"type":"Integer","value":"1"}],"notifications":[]}`,
			`{"txid":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","trigger":"BadTrigger","vmstate":"HALT","gasconsumed":"1","stack":[{"type":"Integer","value":"1"}],"notifications":[]}`,
		}
		for _, str := range errorCases {
			actual := new(AppExecResult)
			err := json.Unmarshal([]byte(str), actual)
			require.Error(t, err)
		}
	})
}
