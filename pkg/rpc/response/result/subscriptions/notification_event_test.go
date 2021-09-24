package subscriptions

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

func TestNotificationEvent_MarshalUnmarshalJSON(t *testing.T) {
	testserdes.MarshalUnmarshalJSON(t, &NotificationEvent{
		Container: util.Uint256{1, 2, 3},
		NotificationEvent: state.NotificationEvent{
			ScriptHash: util.Uint160{4, 5, 6},
			Name:       "alarm",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte("qwerty"))}),
		},
	}, new(NotificationEvent))
}
