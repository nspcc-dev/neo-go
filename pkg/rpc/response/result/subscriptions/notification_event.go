package subscriptions

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NotificationEvent represents wrapper for notification from script execution.
type NotificationEvent struct {
	// Container hash is the hash of script container which is either a block or a transaction.
	Container util.Uint256
	state.NotificationEvent
}

// notificationEventAux is an auxiliary struct for JSON marshalling.
type notificationEventAux struct {
	Container util.Uint256 `json:"container"`
}

// MarshalJSON implements implements json.Marshaler interface.
func (ne *NotificationEvent) MarshalJSON() ([]byte, error) {
	h, err := json.Marshal(&notificationEventAux{
		Container: ne.Container,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hash: %w", err)
	}
	exec, err := json.Marshal(ne.NotificationEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal execution: %w", err)
	}

	if h[len(h)-1] != '}' || exec[0] != '{' {
		return nil, errors.New("can't merge internal jsons")
	}
	h[len(h)-1] = ','
	h = append(h, exec[1:]...)
	return h, nil
}

// UnmarshalJSON implements implements json.Unmarshaler interface.
func (ne *NotificationEvent) UnmarshalJSON(data []byte) error {
	aux := new(notificationEventAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &ne.NotificationEvent); err != nil {
		return err
	}
	ne.Container = aux.Container
	return nil
}
