package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
)

// BlockNotifications represents notifications from a block organized by trigger type.
type BlockNotifications struct {
	PrePersistNotifications  []state.ContainedNotificationEvent `json:"prepersist,omitempty"`
	TxNotifications          []state.ContainedNotificationEvent `json:"transactions,omitempty"`
	PostPersistNotifications []state.ContainedNotificationEvent `json:"postpersist,omitempty"`
}
