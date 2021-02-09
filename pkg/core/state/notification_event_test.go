package state

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
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
	newAer := func() *AppExecResult {
		return &AppExecResult{
			Container: random.Uint256(),
			Execution: Execution{
				Trigger:     1,
				VMState:     vm.HaltState,
				GasConsumed: 10,
				Stack:       []stackitem.Item{stackitem.NewBool(true)},
				Events:      []NotificationEvent{},
			},
		}
	}
	t.Run("halt", func(t *testing.T) {
		appExecResult := newAer()
		appExecResult.VMState = vm.HaltState
		testserdes.EncodeDecodeBinary(t, appExecResult, new(AppExecResult))
	})
	t.Run("fault", func(t *testing.T) {
		appExecResult := newAer()
		appExecResult.VMState = vm.FaultState
		testserdes.EncodeDecodeBinary(t, appExecResult, new(AppExecResult))
	})
	t.Run("with interop", func(t *testing.T) {
		appExecResult := newAer()
		appExecResult.Stack = []stackitem.Item{stackitem.NewInterop(nil)}
		testserdes.EncodeDecodeBinary(t, appExecResult, new(AppExecResult))
	})
	t.Run("recursive reference", func(t *testing.T) {
		var arr = stackitem.NewArray(nil)
		arr.Append(arr)
		appExecResult := newAer()
		appExecResult.Stack = []stackitem.Item{arr, stackitem.NewBool(true), stackitem.NewInterop(123)}

		bs, err := testserdes.EncodeBinary(appExecResult)
		require.NoError(t, err)
		actual := new(AppExecResult)
		require.NoError(t, testserdes.DecodeBinary(bs, actual))
		require.Equal(t, 3, len(actual.Stack))
		require.Nil(t, actual.Stack[0])
		require.Equal(t, true, actual.Stack[1].Value())
		require.Equal(t, stackitem.InteropT, actual.Stack[2].Type())

		bs1, err := testserdes.EncodeBinary(actual)
		require.NoError(t, err)
		require.Equal(t, bs, bs1)
	})
	t.Run("invalid item type", func(t *testing.T) {
		aer := newAer()
		w := io.NewBufBinWriter()
		w.WriteBytes(aer.Container[:])
		w.WriteB(byte(aer.Trigger))
		w.WriteB(byte(aer.VMState))
		w.WriteU64LE(uint64(aer.GasConsumed))
		stackitem.EncodeBinaryStackItem(stackitem.NewBool(true), w.BinWriter)
		require.NoError(t, w.Err)
		require.Error(t, testserdes.DecodeBinary(w.Bytes(), new(AppExecResult)))
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
				Stack:          []stackitem.Item{stackitem.NewBool(true)},
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
		testserdes.MarshalUnmarshalJSON(t, appExecResult, new(AppExecResult))
	})

	t.Run("MarshalJSON recursive reference", func(t *testing.T) {
		arr := stackitem.NewArray(nil)
		arr.Append(arr)
		errAer := &AppExecResult{
			Execution: Execution{
				Trigger: trigger.Application,
				Stack:   []stackitem.Item{arr, stackitem.NewBool(true), stackitem.NewInterop(123)},
			},
		}

		bs, err := json.Marshal(errAer)
		require.NoError(t, err)

		actual := new(AppExecResult)
		require.NoError(t, json.Unmarshal(bs, actual))
		require.Equal(t, 3, len(actual.Stack))
		require.Nil(t, actual.Stack[0])
		require.Equal(t, true, actual.Stack[1].Value())
		require.Equal(t, stackitem.InteropT, actual.Stack[2].Type())

		bs1, err := json.Marshal(actual)
		require.NoError(t, err)
		require.Equal(t, bs, bs1)
	})

	t.Run("UnmarshalJSON error", func(t *testing.T) {
		nilStackCases := []string{
			`{"container":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","trigger":"Application","vmstate":"HALT","gasconsumed":"1","stack":[{"type":"WrongType","value":"1"}],"notifications":[]}`,
		}
		for _, str := range nilStackCases {
			actual := new(AppExecResult)
			err := json.Unmarshal([]byte(str), actual)
			require.NoError(t, err)
			require.Nil(t, actual.Stack)
		}

		errorCases := []string{
			`{"container":"0xBadHash","trigger":"Application","vmstate":"HALT","gasconsumed":"1","stack":[{"type":"Integer","value":"1"}],"notifications":[]}`,
			`{"container":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","trigger":"Application","vmstate":"BadState","gasconsumed":"1","stack":[{"type":"Integer","value":"1"}],"notifications":[]}`,
			`{"container":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","trigger":"BadTrigger","vmstate":"HALT","gasconsumed":"1","stack":[{"type":"Integer","value":"1"}],"notifications":[]}`,
		}
		for _, str := range errorCases {
			actual := new(AppExecResult)
			err := json.Unmarshal([]byte(str), actual)
			require.Error(t, err)
		}
	})
}
