package state

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
	appExecResult := &AppExecResult{
		TxHash:      random.Uint256(),
		Trigger:     1,
		VMState:     "Hault",
		GasConsumed: 10,
		Stack:       []smartcontract.Parameter{},
		Events:      []NotificationEvent{},
	}

	testserdes.EncodeDecodeBinary(t, appExecResult, new(AppExecResult))
}
