package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
)

// BlockNotifications represents notifications from a block organized by
// trigger type.
type BlockNotifications struct {
	// Block-level execution _before_ any transactions.
	OnPersist []state.ContainedNotificationEvent `json:"onpersist,omitzero"`
	// Transaction execution.
	Application []state.ContainedNotificationEvent `json:"application,omitzero"`
	// Block-level execution _after_ all transactions.
	PostPersist []state.ContainedNotificationEvent `json:"postpersist,omitzero"`
}
