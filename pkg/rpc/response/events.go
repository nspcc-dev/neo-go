package response

import (
	"encoding/json"
	"errors"
)

// EventID represents an event type happening on the chain.
type EventID byte

const (
	// InvalidEventID is an invalid event id that is the default value of
	// EventID. It's only used as an initial value similar to nil.
	InvalidEventID EventID = iota
	// BlockEventID is a `block_added` event.
	BlockEventID
	// TransactionEventID corresponds to `transaction_added` event.
	TransactionEventID
	// NotificationEventID represents `notification_from_execution` events.
	NotificationEventID
	// ExecutionEventID is used for `transaction_executed` events.
	ExecutionEventID
	// NotaryRequestEventID is used for `notary_request_event` event.
	NotaryRequestEventID
	// MissedEventID notifies user of missed events.
	MissedEventID EventID = 255
)

// String is a good old Stringer implementation.
func (e EventID) String() string {
	switch e {
	case BlockEventID:
		return "block_added"
	case TransactionEventID:
		return "transaction_added"
	case NotificationEventID:
		return "notification_from_execution"
	case ExecutionEventID:
		return "transaction_executed"
	case NotaryRequestEventID:
		return "notary_request_event"
	case MissedEventID:
		return "event_missed"
	default:
		return "unknown"
	}
}

// GetEventIDFromString converts input string into an EventID if it's possible.
func GetEventIDFromString(s string) (EventID, error) {
	switch s {
	case "block_added":
		return BlockEventID, nil
	case "transaction_added":
		return TransactionEventID, nil
	case "notification_from_execution":
		return NotificationEventID, nil
	case "transaction_executed":
		return ExecutionEventID, nil
	case "notary_request_event":
		return NotaryRequestEventID, nil
	case "event_missed":
		return MissedEventID, nil
	default:
		return 255, errors.New("invalid stream name")
	}
}

// MarshalJSON implements json.Marshaler interface.
func (e EventID) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (e *EventID) UnmarshalJSON(b []byte) error {
	var s string

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	id, err := GetEventIDFromString(s)
	if err != nil {
		return err
	}
	*e = id
	return nil
}
